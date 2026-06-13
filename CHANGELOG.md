# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 1.0.0 (2026-06-13)


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
