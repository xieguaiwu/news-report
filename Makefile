BIN := bin/news-report
GOFLAGS :=

.PHONY: build test vet fmt smoke clean install

build:
	go build $(GOFLAGS) -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# 冒烟测试：少量真实来源 + 短窗口，验证联网抓取链路
smoke: build
	$(BIN) --sources bbc-world,guardian-world,lemonde-international,faz-politik,dw-de,nyt-world \
		--minutes 1440 --limit 2 --total 10 --out markdown --outfile /tmp/news-smoke.md --no-color
	@echo "smoke report: /tmp/news-smoke.md"

install: build
	install -Dm755 $(BIN) $(HOME)/.local/bin/news-report

clean:
	rm -rf bin
