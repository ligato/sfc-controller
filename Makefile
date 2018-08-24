VERSION	:= $(shell git describe --always --tags --dirty)
COMMIT	:= $(shell git rev-parse HEAD)
DATE	:= $(shell date +'%Y-%m-%dT%H:%M%:z')

CNINFRA_CORE := github.com/ligato/sfc-controller/vendor/github.com/ligato/cn-infra/agent
LDFLAGS	= -ldflags '-X $(CNINFRA_CORE).BuildVersion=$(VERSION) -X $(CNINFRA_CORE).CommitHash=$(COMMIT) -X $(CNINFRA_CORE).BuildDate=$(DATE)'

PLUGIN_SOURCES    = "sfc_controller.go"
PLUGIN_BIN        = "sfc_controller.so"
ETCD_CONFIG_FILE  = "etcd/etcd.conf"
KAFKA_CONFIG_FILE = "kafka/kafka.conf"
SFC_CONFIG_FILE   = "sfc.conf"

# generate go structures from proto files & binapi json files
define generate_sources
        $(if $(shell command -v protoc --gogo_out=. 2> /dev/null),$(info gogo/protobuf is installed),$(error gogo/protobuf missing, please install it with go get github.com/gogo/protobuf))
        @echo "# generating sources"
        @cd plugins/controller && go generate -v
        @echo "# done"
endef

# run all tests
define test_only
	@echo "# running unit tests"
	@go test ./tests/gotests/itest
	@echo "# done"
endef

# run all tests with coverage
define test_cover_only
	@echo "# running unit tests with coverage analysis"
	@go test -covermode=count -coverprofile=${COVER_DIR}coverage_unit1.out ./tests/gotests/itest
	@echo "# merging coverage results"
    @cd vendor/github.com/wadey/gocovmerge && go install -v
    @gocovmerge ${COVER_DIR}coverage_unit1.out  > ${COVER_DIR}coverage.out
    @echo "# coverage data generated into ${COVER_DIR}coverage.out"
    @echo "# done"
endef

# run all tests with coverage and display HTML report
define test_cover_html
    $(call test_cover_only)
    @go tool cover -html=${COVER_DIR}coverage.out -o ${COVER_DIR}coverage.html
    @echo "# coverage report generated into ${COVER_DIR}coverage.html"
    @go tool cover -html=${COVER_DIR}coverage.out
endef

# run all tests with coverage and display XML report
define test_cover_xml
	$(call test_cover_only)
    @gocov convert ${COVER_DIR}coverage.out | gocov-xml > ${COVER_DIR}coverage.xml
    @echo "# coverage report generated into ${COVER_DIR}coverage.xml"
endef

# run code analysis
define lint_only
   @echo "# running code analysis"
    @./scripts/static_analysis.sh golint vet
    @echo "# done"
endef

# verify that links in markdown files are valid
# requires npm install -g markdown-link-check
define check_links_only
    @echo "# checking links"
    @./scripts/check_links.sh
    @echo "# done"
endef

# run test examples
define test_examples
    @echo "# TODO Testing examples"
endef

# build-only binaries
define build_only
        @go version
        @echo "# building the sfc controller with plugins"
        @go build -i -v ${LDFLAGS}
        @echo "# done"
endef

# install-only binaries
define install_only
        @echo "# installing sfc controller with plugins"
        @go install

        if test "$(ETCDV3_CONFIG)" != "" ; then \
        echo "# Installing '$(ETCD_CONFIG_FILE)' to '$(ETCDV3_CONFIG)''..."; \
                cp $(ETCD_CONFIG_FILE) $(ETCDV3_CONFIG); \
        fi

        if test "$(KAFKA_CONFIG)" != "" ; then \
        echo "# Installing '$(KAFKA_CONFIG_FILE)' to '$(KAFKA_CONFIG)''..."; \
                cp $(KAFKA_CONFIG_FILE) $(KAFKA_CONFIG); \
        fi

        if test "$(SFC_CONFIG)" != "" ; then \
        echo "# Installing '$(SFC_CONFIG_FILE)' to '$(SFC_CONFIG)''..."; \
                cp $(SFC_CONFIG_FILE) $(SFC_CONFIG); \
        fi

        @echo "# done"
endef

# build binaries
build:
	$(call build_only)

# install binaries
install:
	$(call install_only)

# Get dependency manager tool
get-dep:
	go get -v github.com/golang/dep/cmd/dep

# Install the project's dependencies
dep-install: get-dep
	@echo "# installing project's dependencies"
	dep ensure

# Update the locked versions of all dependencies
dep-update: get-dep
	@echo "# updating all dependencies"
	dep ensure -update

# generate structures
generate:
	$(call generate_sources)

# build & install
all:
	$(call build_only)
	$(call install_only)

# run tests
test:
	@cd bin_api && go test -cover
#       @cd etcd && go test -cover

# print golint suggestions to stderr
lint:
	$(call lint_only)

# report suspicious constructs using go vet tool
govet:
	@./scripts/static_analysis.sh vet

# format code using gofmt tool
format:
	@./scripts/gofmt.sh

# validate links in markdown files
check_links:
	$(call check_links_only)

# clean
clean:
	@echo "# cleaning up the plugin binary"
	@rm -f ${PLUGIN_BIN}
	@echo "# done"

# run smoke tests on examples - TODO
test-examples:
	$(call test_examples)

# run tests with coverage report
test-cover:
	$(call test_cover_only)

# Install yamllint
get-yamllint:
	pip install --user yamllint

# Lint the yaml files
yamllint: get-yamllint
	@echo "=> linting the yaml files"
	yamllint -c .yamllint.yml $(shell git ls-files '*.yaml' '*.yml' | grep -v 'vendor/')

.PHONY: build get-dep dep-install dep-update test lint clean \
	get-yamllint yamllint
