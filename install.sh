#!/usr/bin/env bash
set -euo pipefail

REPO="codemem-io/zesh"
BINARY="zesh"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── detect OS ────────────────────────────────────────────────────────────────
OS="$(uname -s)"
case "$OS" in
  Linux)  OS="linux"  ;;
  Darwin) OS="darwin" ;;
  *)
    echo "error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# ── detect arch ──────────────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# ── resolve version ───────────────────────────────────────────────────────────
if [[ -z "${VERSION:-}" ]]; then
  echo "Fetching latest release..."
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi

if [[ -z "$VERSION" ]]; then
  echo "error: could not determine version. Set VERSION env var or check GitHub releases." >&2
  exit 1
fi

ASSET="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

# ── download ──────────────────────────────────────────────────────────────────
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading ${BINARY} ${VERSION} (${OS}/${ARCH})..."
curl -fsSL "$URL" -o "$TMP/$BINARY"
chmod +x "$TMP/$BINARY"

# ── install binary ────────────────────────────────────────────────────────────
if [[ -w "$INSTALL_DIR" ]]; then
  mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "Installing to $INSTALL_DIR (sudo required)..."
  sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "Installed: $INSTALL_DIR/$BINARY"

# ── shell completion ──────────────────────────────────────────────────────────
install_completion() {
  local shell="$1"
  case "$shell" in
    bash)
      local comp_dir
      if [[ "$OS" == "darwin" ]]; then
        # Homebrew bash-completion
        if [[ -d /usr/local/etc/bash_completion.d ]]; then
          comp_dir="/usr/local/etc/bash_completion.d"
        elif [[ -d /opt/homebrew/etc/bash_completion.d ]]; then
          comp_dir="/opt/homebrew/etc/bash_completion.d"
        else
          comp_dir="$HOME/.bash_completion.d"
          mkdir -p "$comp_dir"
          # Add sourcing to .bashrc/.bash_profile if not already present
          local rc="$HOME/.bash_profile"
          [[ -f "$HOME/.bashrc" ]] && rc="$HOME/.bashrc"
          if ! grep -q "bash_completion.d" "$rc" 2>/dev/null; then
            printf '\n# zesh completions\nfor f in %s/*; do source "$f"; done\n' "$comp_dir" >> "$rc"
          fi
        fi
      else
        comp_dir="/etc/bash_completion.d"
        if [[ ! -w "$comp_dir" ]]; then
          comp_dir="$HOME/.bash_completion.d"
          mkdir -p "$comp_dir"
          local rc="$HOME/.bashrc"
          if ! grep -q "bash_completion.d" "$rc" 2>/dev/null; then
            printf '\n# zesh completions\nfor f in %s/*; do source "$f"; done\n' "$comp_dir" >> "$rc"
          fi
        fi
      fi
      "$INSTALL_DIR/$BINARY" completion bash > "$comp_dir/$BINARY"
      echo "Bash completion installed to $comp_dir/$BINARY"
      ;;

    zsh)
      local comp_dir
      if [[ "$OS" == "darwin" ]]; then
        if [[ -d /usr/local/share/zsh/site-functions ]]; then
          comp_dir="/usr/local/share/zsh/site-functions"
        elif [[ -d /opt/homebrew/share/zsh/site-functions ]]; then
          comp_dir="/opt/homebrew/share/zsh/site-functions"
        else
          comp_dir="${ZDOTDIR:-$HOME}/.zsh/completions"
          mkdir -p "$comp_dir"
          local zshrc="${ZDOTDIR:-$HOME}/.zshrc"
          if ! grep -q "zsh/completions" "$zshrc" 2>/dev/null; then
            printf '\n# zesh completions\nfpath=(%s $fpath)\nautoload -Uz compinit && compinit\n' "$comp_dir" >> "$zshrc"
          fi
        fi
      else
        comp_dir="${ZDOTDIR:-$HOME}/.zsh/completions"
        mkdir -p "$comp_dir"
        local zshrc="${ZDOTDIR:-$HOME}/.zshrc"
        if ! grep -q "zsh/completions" "$zshrc" 2>/dev/null; then
          printf '\n# zesh completions\nfpath=(%s $fpath)\nautoload -Uz compinit && compinit\n' "$comp_dir" >> "$zshrc"
        fi
      fi
      "$INSTALL_DIR/$BINARY" completion zsh > "$comp_dir/_$BINARY"
      echo "Zsh completion installed to $comp_dir/_$BINARY"
      ;;

    fish)
      local comp_dir="$HOME/.config/fish/completions"
      mkdir -p "$comp_dir"
      "$INSTALL_DIR/$BINARY" completion fish > "$comp_dir/$BINARY.fish"
      echo "Fish completion installed to $comp_dir/$BINARY.fish"
      ;;
  esac
}

# detect current shell and install its completion
CURRENT_SHELL="$(basename "${SHELL:-}")"
case "$CURRENT_SHELL" in
  bash | zsh | fish)
    install_completion "$CURRENT_SHELL"
    ;;
  *)
    echo "Shell $CURRENT_SHELL not recognised — skipping completion."
    echo "Run one of these manually:"
    echo "  $BINARY completion bash > /etc/bash_completion.d/$BINARY"
    echo "  $BINARY completion zsh  > \"\${fpath[1]}/_$BINARY\""
    echo "  $BINARY completion fish > ~/.config/fish/completions/$BINARY.fish"
    ;;
esac

echo ""
echo "Done! Run '$BINARY --help' to get started."
echo "Restart your shell (or open a new terminal) to activate completions."
