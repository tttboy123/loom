//go:build darwin && cgo

package credentials

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

static CFStringRef loom_string(const char *bytes, size_t length) {
	return CFStringCreateWithBytes(
		kCFAllocatorDefault,
		(const UInt8 *)bytes,
		(CFIndex)length,
		kCFStringEncodingUTF8,
		false
	);
}

static CFMutableDictionaryRef loom_query(
	const char *service_bytes,
	size_t service_length,
	const char *account_bytes,
	size_t account_length
) {
	CFStringRef service = loom_string(service_bytes, service_length);
	CFStringRef account = loom_string(account_bytes, account_length);
	if (service == NULL || account == NULL) {
		if (service != NULL) CFRelease(service);
		if (account != NULL) CFRelease(account);
		return NULL;
	}
	CFMutableDictionaryRef query = CFDictionaryCreateMutable(
		kCFAllocatorDefault,
		0,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks
	);
	if (query != NULL) {
		CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
		CFDictionarySetValue(query, kSecAttrService, service);
		CFDictionarySetValue(query, kSecAttrAccount, account);
		CFDictionarySetValue(query, kSecAttrSynchronizable, kCFBooleanFalse);
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
		CFDictionarySetValue(
			query,
			kSecUseAuthenticationUI,
			kSecUseAuthenticationUIFail
		);
#pragma clang diagnostic pop
	}
	CFRelease(service);
	CFRelease(account);
	return query;
}

static OSStatus loom_keychain_put(
	const char *service_bytes,
	size_t service_length,
	const char *account_bytes,
	size_t account_length,
	const unsigned char *secret_bytes,
	size_t secret_length
) {
	CFMutableDictionaryRef query = loom_query(
		service_bytes, service_length, account_bytes, account_length
	);
	if (query == NULL) return errSecParam;
	CFDataRef secret = CFDataCreate(
		kCFAllocatorDefault,
		(const UInt8 *)secret_bytes,
		(CFIndex)secret_length
	);
	if (secret == NULL) {
		CFRelease(query);
		return errSecParam;
	}
	CFDictionarySetValue(query, kSecValueData, secret);
	CFDictionarySetValue(
		query,
		kSecAttrAccessible,
		kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
	);
	OSStatus status = SecItemAdd(query, NULL);
	if (status == errSecDuplicateItem) {
		CFDictionaryRemoveValue(query, kSecValueData);
		CFDictionaryRemoveValue(query, kSecAttrAccessible);
		const void *keys[] = {kSecValueData, kSecAttrAccessible};
		const void *values[] = {
			secret,
			kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
		};
		CFDictionaryRef update = CFDictionaryCreate(
			kCFAllocatorDefault,
			keys,
			values,
			2,
			&kCFTypeDictionaryKeyCallBacks,
			&kCFTypeDictionaryValueCallBacks
		);
		if (update == NULL) {
			status = errSecParam;
		} else {
			status = SecItemUpdate(query, update);
			CFRelease(update);
		}
	}
	CFRelease(secret);
	CFRelease(query);
	return status;
}

static OSStatus loom_keychain_read(
	const char *service_bytes,
	size_t service_length,
	const char *account_bytes,
	size_t account_length,
	unsigned char **output,
	size_t *output_length
) {
	*output = NULL;
	*output_length = 0;
	CFMutableDictionaryRef query = loom_query(
		service_bytes, service_length, account_bytes, account_length
	);
	if (query == NULL) return errSecParam;
	CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);
	if (status != errSecSuccess) return status;
	if (result == NULL || CFGetTypeID(result) != CFDataGetTypeID()) {
		if (result != NULL) CFRelease(result);
		return errSecDecode;
	}
	CFDataRef data = (CFDataRef)result;
	CFIndex length = CFDataGetLength(data);
	if (length <= 0) {
		CFRelease(result);
		return errSecDecode;
	}
	unsigned char *copy = (unsigned char *)malloc((size_t)length);
	if (copy == NULL) {
		CFRelease(result);
		return errSecAllocate;
	}
	CFDataGetBytes(
		data,
		CFRangeMake(0, length),
		(UInt8 *)copy
	);
	CFRelease(result);
	*output = copy;
	*output_length = (size_t)length;
	return errSecSuccess;
}

static OSStatus loom_keychain_delete(
	const char *service_bytes,
	size_t service_length,
	const char *account_bytes,
	size_t account_length
) {
	CFMutableDictionaryRef query = loom_query(
		service_bytes, service_length, account_bytes, account_length
	);
	if (query == NULL) return errSecParam;
	OSStatus status = SecItemDelete(query);
	CFRelease(query);
	return status;
}
*/
import "C"

import (
	"context"
	"unsafe"
)

const defaultKeychainServiceName = "com.earendilworks.loom.provider"

type KeychainStoreConfig struct {
	ServiceName string
}

type KeychainStore struct {
	serviceName string
}

func NewKeychainStore(config KeychainStoreConfig) (*KeychainStore, error) {
	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = defaultKeychainServiceName
	}
	if serviceName != defaultKeychainServiceName {
		return nil, ErrCredentialStoreUnavailable
	}
	return &KeychainStore{serviceName: serviceName}, nil
}

func (store *KeychainStore) Put(
	ctx context.Context,
	reference string,
	secret []byte,
) error {
	if err := validateKeychainInput(ctx, store, reference); err != nil ||
		len(secret) == 0 ||
		len(secret) > 8192 {
		return ErrCredentialStoreUnavailable
	}
	status := C.loom_keychain_put(
		(*C.char)(unsafe.Pointer(unsafe.StringData(store.serviceName))),
		C.size_t(len(store.serviceName)),
		(*C.char)(unsafe.Pointer(unsafe.StringData(reference))),
		C.size_t(len(reference)),
		(*C.uchar)(unsafe.Pointer(unsafe.SliceData(secret))),
		C.size_t(len(secret)),
	)
	return keychainStatusError(status)
}

func (store *KeychainStore) Read(
	ctx context.Context,
	reference string,
) ([]byte, error) {
	if err := validateKeychainInput(ctx, store, reference); err != nil {
		return nil, err
	}
	var output *C.uchar
	var outputLength C.size_t
	status := C.loom_keychain_read(
		(*C.char)(unsafe.Pointer(unsafe.StringData(store.serviceName))),
		C.size_t(len(store.serviceName)),
		(*C.char)(unsafe.Pointer(unsafe.StringData(reference))),
		C.size_t(len(reference)),
		&output,
		&outputLength,
	)
	if err := keychainStatusError(status); err != nil {
		return nil, err
	}
	if output == nil || outputLength == 0 || outputLength > 8192 {
		if output != nil {
			C.free(unsafe.Pointer(output))
		}
		return nil, ErrCredentialStoreUnavailable
	}
	defer C.free(unsafe.Pointer(output))
	value := C.GoBytes(unsafe.Pointer(output), C.int(outputLength))
	return value, nil
}

func (store *KeychainStore) Delete(
	ctx context.Context,
	reference string,
) error {
	if err := validateKeychainInput(ctx, store, reference); err != nil {
		return err
	}
	status := C.loom_keychain_delete(
		(*C.char)(unsafe.Pointer(unsafe.StringData(store.serviceName))),
		C.size_t(len(store.serviceName)),
		(*C.char)(unsafe.Pointer(unsafe.StringData(reference))),
		C.size_t(len(reference)),
	)
	return keychainStatusError(status)
}

func validateKeychainInput(
	ctx context.Context,
	store *KeychainStore,
	reference string,
) error {
	if ctx == nil {
		return ErrCredentialStoreUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if store == nil ||
		!validCredentialIdentifier(store.serviceName, 128) ||
		!validCredentialIdentifier(reference, 128) {
		return ErrCredentialStoreUnavailable
	}
	return nil
}

func keychainStatusError(status C.OSStatus) error {
	switch status {
	case C.errSecSuccess:
		return nil
	case C.errSecItemNotFound:
		return ErrCredentialNotFound
	case C.errSecAuthFailed, C.errSecInteractionNotAllowed, C.errSecUserCanceled:
		return ErrCredentialStoreDenied
	default:
		return ErrCredentialStoreUnavailable
	}
}
