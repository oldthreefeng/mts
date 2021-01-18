LDFLAGS += -X "github.com/oldthreefeng/mts/cmd.Buildstamp=$(shell date -u '+%Y-%m-%d %H:%M:%S %Z')"
LDFLAGS += -X "github.com/oldthreefeng/mts/cmd.Githash=$(shell git rev-parse --short HEAD)"
LDFLAGS += -X "github.com/oldthreefeng/mts/cmd.Goversion=$(shell go version)"
BRANCH := $(shell git symbolic-ref HEAD 2>/dev/null | cut -d"/" -f 3)
BUILD := $(shell git rev-parse --short HEAD)
VERSION = $(BRANCH)-$(BUILD)

BASEPATH := $(shell pwd)
CGO_ENABLED = 0
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOMOD = $(GOCMD) mod
GOPATH = $(shell go env GOPATH)
GOFILES = $(shell find . -name "*.go" -type f )

NAME := mts
DIRNAME := output/bin
GOBIN := $(GOPATH)/bin/
WLSBIN := /mnt/c/Go/bin/
SRCFILE= main.go
SOFTWARENAME=$(NAME)-$(VERSION)

PLATFORMS := darwin linux windows

.PHONY: run
run: deps
	$(GOBUILD)  -ldflags '$(LDFLAGS)'  -o $(NAME) $(SRCFILE) 
	./$(NAME)

.PHONY: fmt
fmt:
	@gofmt -s -w ${GOFILES}	

.PHONY: test
test: deps
	$(GOTEST) -v ./...

.PHONY: deps
deps:
	$(GOMOD) tidy
	$(GOMOD) download

.PHONY: release
release: darwin linux

BUILDDIR:=$(BASEPATH)/output

.PHONY:Asset
Asset:
	@[ -d $(BUILDDIR) ] || mkdir -p $(BUILDDIR)
	@[ -d $(DIRNAME) ] || mkdir -p $(DIRNAME)

.PHONY: $(PLATFORMS)
$(PLATFORMS): Asset deps
	@echo "编译" $@
	GOOS=$@ GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on GOPROXY=https://goproxy.cn $(GOBUILD) -ldflags '$(LDFLAGS)'  -o $(NAME) $(SRCFILE)
	# cp -f $(NAME) $(DIRNAME)
	cp -f $(NAME) $(GOBIN)
	# tar czvf $(BUILDDIR)/$(SOFTWARENAME)-$@-amd64.tar.gz $(DIRNAME)
	(test -d $(WLSBIN) && cp -f $(NAME) $(WLSBIN)$(NAME).exe) || true

.PHONY: clean
clean:
	-rm -rf $(NAME)
	# -rm -rf $(DIRNAME)
	# -rm -rf $(BUILDDIR)
	-rm -rf $(GOBIN)$(NAME)
	-rm -rf $(WLSBIN)$(NAME).exe

.PHONY: push
push: clean
	-git push origin master
	-git push github master

