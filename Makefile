OUTPUT_BASE="build"

.PHONY: build serve
build:
	go run . render -a example/asset -l example/website -l example/core -o build website/content/v1/domain/blog
	cp example/public/robots.txt $(OUTPUT_BASE)
	cat example/public/css/*.css > $(OUTPUT_BASE)/style.css

serve:
	ran -l -r build