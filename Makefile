OUTPUT_BASE="build"

.PHONY: example clean serve
example: clean
	go run . render -a example/asset -l example/website -l example/templates -o build website/page/v1/domain/blog
	cp -R example/public/* $(OUTPUT_BASE)

clean:
	mkdir -p build
	find build -type f -name "*.html" -exec rm {} +

serve:
	ran -l -r build