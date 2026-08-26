package ccswitch

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"loom-pi-rebuild/internal/credentials"

	_ "modernc.org/sqlite"
)

const (
	ProtocolOpenAIResponses    = credentials.ImportProtocolOpenAIResponses
	ImportExactProvider        = credentials.ImportModeExactProvider
	ImportCustomEndpointReview = credentials.ImportModeCustomEndpointReview

	maximumConfigBytes       = 128 << 10
	maximumModelCatalogBytes = 256 << 10
	maximumSecretBytes       = 8192
)

var (
	ErrUnsafeSource         = errors.New("unsafe CC Switch source")
	ErrSourceUnavailable    = errors.New("CC Switch source unavailable")
	ErrCandidateUnavailable = errors.New("CC Switch candidate unavailable")
)

// Candidate is intentionally non-secret. It is safe to project into setup UI,
// but it is not an execution profile and does not authorize credential egress.
type Candidate = credentials.ImportCandidate

type sourceIdentity struct {
	device uint64
	inode  uint64
	uid    uint32
}

type Source struct {
	path     string
	identity sourceIdentity
}

type sourceCandidate struct {
	candidate Candidate
	rowID     string
	appType   string
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewSource(path string) (*Source, error) {
	identity, err := validateSource(path)
	if err != nil {
		return nil, err
	}
	return &Source{path: path, identity: identity}, nil
}

func (source *Source) Discover(ctx context.Context) ([]Candidate, error) {
	if source == nil || ctx == nil {
		return nil, ErrSourceUnavailable
	}
	database, err := source.open(ctx)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	records, err := scanCandidates(ctx, database)
	if err != nil {
		return nil, err
	}
	result := make([]Candidate, 0, len(records))
	for _, record := range records {
		result = append(result, copyCandidate(record.candidate))
	}
	if result == nil {
		result = []Candidate{}
	}
	return result, nil
}

// UseAPIKey reads only the selected API-key field, lends it for one bounded
// callback, and zeroizes the mutable buffer before returning. OAuth tokens are
// never selected by this path.
func (source *Source) UseAPIKey(
	ctx context.Context,
	candidateID string,
	use func(context.Context, []byte) error,
) error {
	if source == nil || ctx == nil || !validCandidateID(candidateID) || use == nil {
		return ErrCandidateUnavailable
	}
	database, err := source.open(ctx)
	if err != nil {
		return err
	}
	defer database.Close()
	tx, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ErrSourceUnavailable
	}
	defer tx.Rollback()
	records, err := scanCandidates(ctx, tx)
	if err != nil {
		return err
	}
	var selected *sourceCandidate
	for index := range records {
		if records[index].candidate.CandidateID == candidateID {
			selected = &records[index]
			break
		}
	}
	if selected == nil {
		return ErrCandidateUnavailable
	}
	var secret []byte
	err = tx.QueryRowContext(
		ctx,
		`SELECT CAST(json_extract(settings_config, '$.auth."OPENAI_API_KEY"') AS BLOB)
FROM providers WHERE id = ? AND app_type = ?`,
		selected.rowID, selected.appType,
	).Scan(&secret)
	if err != nil || len(secret) == 0 || len(secret) > maximumSecretBytes ||
		!utf8.Valid(secret) || containsControl(secret) {
		zero(secret)
		return ErrCandidateUnavailable
	}
	defer zero(secret)
	return use(ctx, secret)
}

func (source *Source) open(ctx context.Context) (*sql.DB, error) {
	identity, err := validateSource(source.path)
	if err != nil || identity != source.identity {
		return nil, ErrUnsafeSource
	}
	values := url.Values{}
	values.Add("mode", "ro")
	values.Add("_pragma", "query_only(1)")
	values.Add("_pragma", "busy_timeout(1000)")
	uri := url.URL{Scheme: "file", Path: source.path, RawQuery: values.Encode()}
	database, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, ErrSourceUnavailable
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, ErrSourceUnavailable
	}
	current, err := validateSource(source.path)
	if err != nil || current != identity {
		_ = database.Close()
		return nil, ErrUnsafeSource
	}
	if err := validateSchema(ctx, database); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func validateSource(path string) (sourceIdentity, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return sourceIdentity{}, ErrUnsafeSource
	}
	parent := filepath.Dir(path)
	parentInfo, err := os.Lstat(parent)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 ||
		parentInfo.Mode().Perm()&0o077 != 0 || !ownedByCurrentUser(parentInfo) {
		return sourceIdentity{}, ErrUnsafeSource
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm()&0o077 != 0 || !ownedByCurrentUser(info) {
		return sourceIdentity{}, ErrUnsafeSource
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return sourceIdentity{}, ErrUnsafeSource
	}
	return sourceIdentity{
		device: uint64(stat.Dev), inode: uint64(stat.Ino), uid: stat.Uid,
	}, nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid()
}

func validateSchema(ctx context.Context, database queryer) error {
	rows, err := database.QueryContext(ctx, `PRAGMA table_info(providers)`)
	if err != nil {
		return ErrSourceUnavailable
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var sequence int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(
			&sequence, &name, &dataType, &notNull, &defaultValue, &primaryKey,
		); err != nil {
			return ErrSourceUnavailable
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return ErrSourceUnavailable
	}
	for _, required := range []string{
		"id", "app_type", "name", "settings_config", "is_current",
	} {
		if !columns[required] {
			return ErrSourceUnavailable
		}
	}
	return nil
}

func scanCandidates(ctx context.Context, database queryer) ([]sourceCandidate, error) {
	rows, err := database.QueryContext(ctx, `
SELECT id, app_type, name, is_current,
       CASE WHEN json_type(settings_config, '$.auth."OPENAI_API_KEY"') = 'text'
                  AND length(json_extract(settings_config, '$.auth."OPENAI_API_KEY"')) BETWEEN 1 AND 8192
            THEN 1 ELSE 0 END,
       COALESCE(json_extract(settings_config, '$.config'), ''),
       COALESCE(json_extract(settings_config, '$.modelCatalog'), '{}')
FROM providers
WHERE app_type = 'codex'
ORDER BY name, id`)
	if err != nil {
		return nil, ErrSourceUnavailable
	}
	defer rows.Close()
	result := []sourceCandidate{}
	for rows.Next() {
		var rowID, appType, name, config, modelCatalog string
		var current, hasAPIKey int
		if err := rows.Scan(
			&rowID, &appType, &name, &current, &hasAPIKey, &config, &modelCatalog,
		); err != nil {
			return nil, ErrSourceUnavailable
		}
		if hasAPIKey != 1 || len(config) == 0 || len(config) > maximumConfigBytes ||
			len(modelCatalog) > maximumModelCatalogBytes {
			continue
		}
		candidate, ok := buildCandidate(
			rowID, appType, name, current == 1, config, modelCatalog,
		)
		if ok {
			result = append(result, sourceCandidate{
				candidate: candidate, rowID: rowID, appType: appType,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, ErrSourceUnavailable
	}
	return result, nil
}

type parsedConfig struct {
	provider string
	model    string
	sections map[string]map[string]string
}

func buildCandidate(
	rowID, appType, name string,
	current bool,
	config, modelCatalog string,
) (Candidate, bool) {
	parsed, ok := parseConfig(config)
	if !ok || parsed.provider == "" || parsed.model == "" {
		return Candidate{}, false
	}
	providerConfig := parsed.sections["model_providers."+parsed.provider]
	endpoint, ok := safeEndpoint(providerConfig["base_url"])
	if !ok || providerConfig["wire_api"] != "responses" {
		return Candidate{}, false
	}
	models := parseModels(modelCatalog, parsed.model)
	if len(models) == 0 {
		return Candidate{}, false
	}
	target, mode := classifyEndpoint(endpoint)
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"loom/ccswitch-candidate/v1", appType, rowID, name, endpoint,
		ProtocolOpenAIResponses, strings.Join(models, "\x1f"), target, mode,
	}, "\x00")))
	candidate, err := credentials.BindImportCandidate(Candidate{
		CandidateID: hex.EncodeToString(digest[:]), SourceApplication: "CC Switch",
		DisplayName: name, TargetProviderID: target, Protocol: ProtocolOpenAIResponses,
		Endpoint: endpoint, ModelIDs: models, ImportMode: mode, Current: current,
		CredentialAvailable: true,
	})
	return candidate, err == nil
}

func parseConfig(value string) (parsedConfig, bool) {
	result := parsedConfig{sections: map[string]map[string]string{"": {}}}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(value))
	scanner.Buffer(make([]byte, 4096), maximumConfigBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section == "" {
				return parsedConfig{}, false
			}
			if result.sections[section] == nil {
				result.sections[section] = map[string]string{}
			}
			continue
		}
		key, raw, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		decoded, ok := quotedValue(strings.TrimSpace(raw))
		if !ok {
			continue
		}
		result.sections[section][key] = decoded
		if section == "" {
			switch key {
			case "model_provider":
				result.provider = decoded
			case "model":
				result.model = decoded
			}
		}
	}
	if scanner.Err() != nil {
		return parsedConfig{}, false
	}
	return result, true
}

func quotedValue(value string) (string, bool) {
	if len(value) < 2 || value[0] != '"' {
		return "", false
	}
	decoded, err := strconv.Unquote(value)
	if err != nil || decoded == "" || !utf8.ValidString(decoded) ||
		strings.IndexByte(decoded, 0) >= 0 {
		return "", false
	}
	return decoded, true
}

func parseModels(catalog, selected string) []string {
	models := []string{}
	if validModelID(selected) {
		models = append(models, selected)
	}
	var decoded struct {
		Models []struct {
			Model string `json:"model"`
		} `json:"models"`
	}
	if len(catalog) <= maximumModelCatalogBytes && json.Unmarshal([]byte(catalog), &decoded) == nil {
		for _, item := range decoded.Models {
			if validModelID(item.Model) {
				models = append(models, item.Model)
			}
		}
	}
	seen := map[string]bool{}
	unique := models[:0]
	for _, model := range models {
		if !seen[model] {
			seen[model] = true
			unique = append(unique, model)
		}
	}
	if len(unique) > 64 {
		unique = unique[:64]
	}
	return unique
}

func validModelID(value string) bool {
	if value == "" || len(value) > 256 || value != strings.TrimSpace(value) ||
		!utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func safeEndpoint(value string) (string, bool) {
	value = strings.TrimSuffix(strings.TrimSpace(value), "/")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" || net.ParseIP(hostname) != nil || hostname == "localhost" ||
		strings.HasSuffix(hostname, ".localhost") || strings.HasSuffix(hostname, ".local") ||
		strings.HasSuffix(hostname, ".internal") || !validPublicHostname(hostname) {
		return "", false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", false
	}
	parsed.Scheme = "https"
	parsed.Host = hostname
	if parsed.Port() == "443" {
		parsed.Host = hostname
	}
	return strings.TrimSuffix(parsed.String(), "/"), true
}

func validPublicHostname(value string) bool {
	if len(value) > 253 || !strings.Contains(value, ".") ||
		strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") ||
			strings.HasSuffix(label, "-") {
			return false
		}
		for _, character := range label {
			if character < 'a' || character > 'z' {
				if character < '0' || character > '9' {
					if character != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func classifyEndpoint(endpoint string) (string, string) {
	switch endpoint {
	case "https://api.deepseek.com":
		return "deepseek", ImportExactProvider
	case "https://api.minimaxi.com/v1":
		return "minimax", ImportExactProvider
	case "https://api.moonshot.cn/v1":
		return "kimi", ImportExactProvider
	default:
		return "custom-openai", ImportCustomEndpointReview
	}
}

func validCandidateID(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func copyCandidate(value Candidate) Candidate {
	value.ModelIDs = append([]string(nil), value.ModelIDs...)
	if value.ModelIDs == nil {
		value.ModelIDs = []string{}
	}
	return value
}

func containsControl(value []byte) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
