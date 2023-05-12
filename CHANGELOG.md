# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [1.4.0] - 2023-05-12
### Added
- Add support for `watchedNamespaces` and `watchedPodNamePrefixes` [#14](https://github.com/airwallex/k8s-pod-restart-info-collector/issues/14)

## [1.3.0] - 2023-05-11
### Added
- Add `ignoreRestartsWithExitCodeZero` flag to ignore restart events with an exit code of 0 [#22](https://github.com/airwallex/k8s-pod-restart-info-collector/issues/22)

## [1.2.1] - 2023-04-27
### Fixed
- Container resource specs showing wrong values [#26](https://github.com/airwallex/k8s-pod-restart-info-collector/issues/26)

### Improved
- Add backticks to format slack message nicely [#25](https://github.com/airwallex/k8s-pod-restart-info-collector/issues/25)

## [1.2.0] - 2023-01-03
### Added
- Parameterize pod restart count

## [1.1.0] - 2022-09-19
### Added
- Support ignoring specific namespaces and pods 

## [1.0.0] - 2022-08-29
### Added
- Initial release as Open-Source under the Apache License v2.0