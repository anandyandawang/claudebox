.PHONY: test test-unit test-integration test-all setup-test-deps clean

test: test-unit

test-unit: setup-test-deps
	./tests/bats/bin/bats tests/unit/*.bats

test-integration: setup-test-deps
	./tests/bats/bin/bats tests/integration/*.bats

test-all: test-unit test-integration

setup-test-deps:
	@./tests/setup_test_deps.sh

clean:
	rm -rf tests/bats tests/test_helper/bats-support tests/test_helper/bats-assert
