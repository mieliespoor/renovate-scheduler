# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.0](https://github.com/mieliespoor/renovate-scheduler/compare/v1.2.4...v1.3.0) (2026-10-12)


### Features

* add support for additional Kubernetes objects in Helm chart ([12e4be1](https://github.com/mieliespoor/renovate-scheduler/commit/12e4be1c4e3ca0ec195537999883041fd915954d))
* add support for podLabels in Renovate workload configuration ([a7333f5](https://github.com/mieliespoor/renovate-scheduler/commit/a7333f51519deea1e3472222c23573432be2bb17))


### Bug Fixes

* **deps:** update golang.org/x/net, golang.org/x/sys, golang.org/x/term, and golang.org/x/text dependencies to latest versions ([b93a64f](https://github.com/mieliespoor/renovate-scheduler/commit/b93a64f85488be78a780b5d8fc59859d47e47bd6))
* **deps:** update golang.org/x/net, golang.org/x/sys, golang.org/x/term, and golang.org/x/text dependencies to latest versions ([717fb41](https://github.com/mieliespoor/renovate-scheduler/commit/717fb41cdb68a79e2f7ddcf3f58990a590499ebf))
* **deps:** update golang.org/x/text dependency to v0.41.0 ([edf57a1](https://github.com/mieliespoor/renovate-scheduler/commit/edf57a121ce3db1b9a9897319a108867225623bd))

## [1.2.4](https://github.com/mieliespoor/renovate-scheduler/compare/v1.2.3...v1.2.4) (2026-10-08)


### Bug Fixes

* ensure in-progress claims are cleared on reload and complete in-flight runs before shutdown ([fe1f3d2](https://github.com/mieliespoor/renovate-scheduler/commit/fe1f3d2b37f72b308e22173d120290ff7fee3acb))

## [1.2.3](https://github.com/mieliespoor/renovate-scheduler/compare/v1.2.2...v1.2.3) (2026-10-07)


### Bug Fixes

* add checksum annotation for config in deployment template ([5253f1b](https://github.com/mieliespoor/renovate-scheduler/commit/5253f1ba3ca26adb4596ded10d8b1d7c5d798f1f))
* add deployment strategy type to deployment.yaml ([8edb773](https://github.com/mieliespoor/renovate-scheduler/commit/8edb7730efa100f1221471feb3868ef6923aa079))
* update args and volume mount paths for scheduler container in deployment.yaml ([3950734](https://github.com/mieliespoor/renovate-scheduler/commit/3950734927c30ea11f7fd055f8a14f867df7f1b1))

## [1.2.2](https://github.com/mieliespoor/renovate-scheduler/compare/v1.2.1...v1.2.2) (2026-10-06)


### Bug Fixes

* add resource requests and limits configuration for Renovate Job containers ([fb5d121](https://github.com/mieliespoor/renovate-scheduler/commit/fb5d1219864eefc4cce07cf622a7d4179d3e3330))
* add workload configuration for Renovate Job in Helm chart and update related code ([a7dea0d](https://github.com/mieliespoor/renovate-scheduler/commit/a7dea0d9c77d23843ccd0ef22b183d26c13369ae))

## [1.2.1](https://github.com/mieliespoor/renovate-scheduler/compare/v1.2.0...v1.2.1) (2026-10-05)


### Bug Fixes

* add resource configuration for scheduler container in values.yaml and deployment.yaml ([bf43c1c](https://github.com/mieliespoor/renovate-scheduler/commit/bf43c1cbcaa164282d74af0cf50c923ab537a20a))

## [v1.2.0](https://github.com/mieliespoor/renovate-scheduler/compare/v1.1.0...v1.2.0) (2026-09-18)


### Features

* change license to Apache-2.0. Update README to include license information ([25d2efb](https://github.com/mieliespoor/renovate-scheduler/commit/25d2efb47b72056fecaf1d488ce89c004238afa1))

## [v1.1.0](https://github.com/mieliespoor/renovate-scheduler/compare/v1.0.0...v1.1.0) (2026-09-18)


### Features

* add helm chart and required changes ([#30](https://github.com/mieliespoor/renovate-scheduler/issues/30)) ([613a328](https://github.com/mieliespoor/renovate-scheduler/commit/613a3282b0238281ae475f0f2ff0c021dbb0b675))

## [1.0.0] (2026-06-13)


### Features

* Implement core functionality for renovate-scheduler ([f3ca36f](https://github.com/mieliespoor/renovate-scheduler/commit/f3ca36f464db30e7f95ba0c24d96139cff5e9b0e))


### Bug Fixes

* resolve data race in dispatchLoop on context cancellation ([91c1b56](https://github.com/mieliespoor/renovate-scheduler/commit/91c1b56160f902278331a35f575d202174de1b60))
* use defer wg.Wait() in dispatchLoop for robustness ([638bdc5](https://github.com/mieliespoor/renovate-scheduler/commit/638bdc58bdee5ebd762cb250bf412d7bf6693e7b))
* wait for in-flight goroutines in dispatchLoop to prevent data race ([7c35642](https://github.com/mieliespoor/renovate-scheduler/commit/7c35642acc05bff697570d47b1d953834d5fd5b3))

## [Unreleased]

### Added
- Initial GitHub Actions CI/CD pipeline setup

### Changed

### Deprecated

### Removed

### Fixed

### Security
