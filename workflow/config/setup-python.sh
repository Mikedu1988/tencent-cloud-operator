#!/usr/bin/env bash
# shellcheck source=../lib/setup.sh
source "$(dirname "${BASH_SOURCE[0]}")/../lib/setup.sh" || exit "${EXIT_FAILED_TO_SOURCE}"

# Arrange for an isolated best-practice python environment via pyenv*.
# Read https://opensource.com/article/19/5/python-3-default-mac at some length.

install_pyenv() {
  log_info "Installing python via pyenv-installer (https://github.com/pyenv/pyenv-installer#installation--update--uninstallation) to avoid taking a dependency on system's default python for really good python ecosystem reasons. See also https://opensource.com/article/19/5/python-3-default-mac \"What we should do\" section at the end (after all the \"We could do x but don't\" parts)..."
  curl -L https://github.com/pyenv/pyenv-installer/raw/master/bin/pyenv-installer | bash
  log_warn "pyenv was just installed; please:"
  log_warn "1. restart your shell(s)."
  log_warn "2. re-run \`workflow/setup.sh\` to get further."
}

log_info "Arranging for python's platform-specific dependencies to be installed. Refer to https://github.com/pyenv/pyenv/wiki/common-build-problems if interested."
if [[ "${OSTYPE}" == *"darwin"* ]]; then
  command -v "brew" >"/dev/null" || log_fatal "brew not present in PATH. Install it: https://brew.sh/"
  log_info "Installing dependencies from homebrew..."
  brew install "readline" "xz"
elif [[ "${OSTYPE}" == *"linux"* ]]; then
  log_info "Installing dependencies from apt (on the optimistic expectation you have ubuntu/debian;"
  sudo apt-get update &&
    sudo apt-get install -y \
      build-essential \
      curl \
      git \
      libbz2-dev \
      libffi-dev \
      liblzma-dev \
      libncurses5-dev \
      libncursesw5-dev \
      libreadline-dev \
      libsqlite3-dev \
      libssl-dev \
      llvm \
      make \
      python-openssl \
      tk-dev \
      wget \
      xz-utils \
      zlib1g-dev
else
  log_error "Unsupported \$OSTYPE: ${OSTYPE}"
  log_error "You can continue, Press [enter] to continue."
  read -rp
fi
if ! command -v pyenv >"/dev/null"; then
  install_pyenv
  exit 2
fi
pyenv_installer_dir="${HOME}/.pyenv/plugins/pyenv-installer"
if ! [[ -d "${pyenv_installer_dir}" ]]; then
  log_error "pyenv is installed, but not via pyenv-installer (${pyenv_installer_dir} did not exist)."
  log_fatal "Please remove your alternate install of pyenv, and run this script again - alternative installations are not supported. Supported: https://github.com/pyenv/pyenv-installer#installation--update--uninstallation."
fi

log_info "Installing python version from \"$(pwd)/.python-version\" if necessary..."
if ! pyenv update >"/dev/null"; then
  log_fatal "pyenv-build must be installed. Install it via pyenv-installer: https://github.com/pyenv/pyenv-installer#installation--update--uninstallation"
fi
log_info "pyenv update"
pyenv update
log_info "pyenv install --skip-existing"
pyenv install --skip-existing "$(pyenv local)"
if ! pyenv virtualenvs >"/dev/null"; then
  log_fatal "pyenv-virtualenv must be installed. Install it via pyenv-installer: https://github.com/pyenv/pyenv-installer#installation--update--uninstallation"
fi
venv_name="$(basename "$(pwd)")"
if ! pyenv virtualenvs | grep "${venv_name}"; then
  log_info "Making python virtualenv for current git worktree (${venv_name})..."
  pyenv virtualenv "${venv_name}"
fi
