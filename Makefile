PYTHON_VERSION ?= "3.13"

#
# Environment setup
#
.PHONY: .env
.env:
	@poetry env use $(PYTHON_VERSION)

.PHONY: install
install: .env
	@poetry install --no-root