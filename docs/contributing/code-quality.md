# Code Quality
* Unit tests, integration tests, and linting run in the
[CI pipeline](https://dev.azure.com/michaelsethperel/docker-lock/_build?definitionId=4)
on pull requests.
* To format your code: `./tools/format.sh`
* To lint your code: `./tools/lint.sh`
* To run unit tests: `./tools/unittest.sh`
* To generate a coverage report: `./tools/coverage.sh`
* To view the coverage report on your browser, open a console, but not in
docker, run `go tool cover -html=coverage.out`