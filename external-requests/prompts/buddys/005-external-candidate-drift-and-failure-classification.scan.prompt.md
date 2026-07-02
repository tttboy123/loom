Review `005-external-candidate-drift-and-failure-classification.request.md` as a generalized Loom capability request from an external project.

Decide whether Loom should intake this request as a generic execution-engine improvement.

Focus on:

1. whether candidate drift detection can be made generic without adopting Buddys-specific backlog or docs rules
2. whether structured failure kinds would help any external queue runner, not just Buddys
3. whether retry/cooldown hints can remain generic and machine-readable

Do not rewrite Loom around Buddys. Accept only the generalized execution-engine parts.
