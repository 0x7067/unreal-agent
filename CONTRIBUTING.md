This repository contains selected components from a larger internal codebase.

We intend to share more components, features, and tools over time, as we evaluate their reliability, token usage, and impact on agent behavior.

We welcome GitHub issues with your questions, feature requests, and bug reports.

Unfortunately, we don't currently have the capacity to review and merge pull requests.


Fork proof policy: Bend 2 laws are authoritative. Read
[the formal verification policy](docs/formal/README.md) before changing modeled
behavior. Run `make proof-test prove proof-regressions test check build`; do not weaken
laws or introduce unsafe proof exceptions to pass a check. `make prove` checks
protocol models and actual-source restricted-semantics contracts on a stable tree.
It is not a whole-Go proof; `make proof-report` lists remaining declarations and
semantic obligations. Preserve legacy proof evidence until a validated replacement.
