.PHONY: test setup-test-deps clean

test: setup-test-deps
	./tests/bats/bin/bats tests/*.bats

setup-test-deps:
	@./tests/setup_test_deps.sh

clean:
	rm -rf tests/bats tests/test_helper/bats-support tests/test_helper/bats-assert
