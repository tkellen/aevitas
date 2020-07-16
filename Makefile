.PHONY: example site
example:
	rm -rf build/test
	go run . render -l example -o build/test website/domain/v1/default/example

site:
	rm -rf build/goingslowly/20* build/goingslowly/tag
	memorybox index | go run . render -l example/templates -l resources -o build/goingslowly website/domain/v1/goingslowly/journal

