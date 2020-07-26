SHELL=/bin/bash
.PHONY: example site clean serve
example: clean
	go run . render -a example/asset -l example/website -l example/templates -o build website/domain/v1/default/example

site: clean
	memorybox index | go run . render -a ~/memorybox -l example/templates -l resources -o build website/domain/v1/goingslowly/journal

clean:
	find build -type f -name "*.html" -exec rm {} +

serve:
	ran -l -r build