.PHONY: build
build:
	/bin/bash -c "(find ./resources/goingslowly/pages -type f | xargs cat ; memorybox index; find resources -name '*.yml' | xargs spruce json) | go run ./ goingslowly/website/domain/v1/journal"

