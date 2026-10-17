#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
GO=${GO:-go}
: "${FULL_TAGS:?use make check-modules to supply the implemented tag list}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
export GOWORK=off
for tags in '' docker_sandboxes containers local_ports remote_http ports_over_ssh ports_over_relay built_in_relay ports_over_relay,built_in_relay local_ports,remote_http local_ports,ports_over_relay docker_sandboxes,ports_over_relay containers,ports_over_relay remote_http,ports_over_relay ports_over_ssh,ports_over_relay "$FULL_TAGS"; do
  echo "Checking modules: ${tags:-core}"
  "$GO" test -race -tags "$tags" ./... > "$work/test.log" 2>&1 || { cat "$work/test.log"; exit 1; }
  "$GO" vet -tags "$tags" ./...
  "$GO" build -tags "$tags" -o "$work/zelport" ./cmd/zelport
  "$GO" list -deps -tags "$tags" ./cmd/zelport > "$work/deps"
  for module in docker_sandboxes containers local_ports remote_http ports_over_ssh ports_over_relay built_in_relay; do
    case ",$tags," in
      *",$module,"*) rg -q "^github.com/zelport/zelport/internal/modules/$module$" "$work/deps" ;;
      *) if rg -q "^github.com/zelport/zelport/internal/modules/$module$" "$work/deps"; then echo "Excluded module was linked: $module"; exit 1; fi ;;
    esac
  done
  case ",$tags," in
    *",ports_over_relay,"*|*",built_in_relay,"*) ;;
    *) if rg -q 'github.com/zelport/zelport-(sdk-go|relay)|github.com/(coder/websocket|flynn/noise|hashicorp/yamux)' "$work/deps"; then echo 'Excluded relay dependency was linked'; exit 1; fi ;;
  esac
  case ",$tags," in
    *",built_in_relay,"*) ;;
    *) if rg -q 'github.com/zelport/zelport-relay/relay$' "$work/deps"; then echo 'Excluded built_in_relay server was linked'; exit 1; fi ;;
  esac
  case ",$tags," in
    *",ports_over_relay,"*) ;;
    *) if rg -q 'github.com/zelport/zelport-sdk-go/client$|github.com/hashicorp/yamux' "$work/deps"; then echo 'Excluded built_in_relay client was linked'; exit 1; fi ;;
  esac
  if [ -z "$tags" ]; then
    if rg -q 'github.com/zelport/zelport/internal/(modules/|modulekit|sbx|containers|sshpipe|relay)|github.com/(coder/websocket|flynn/noise|hashicorp/yamux)' "$work/deps"; then
      echo 'Core imports a feature dependency'; exit 1
    fi
  fi
  "$work/zelport" --version
 done
