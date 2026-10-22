#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
GO=${GO:-go}
modules='local_ports remote_http ports_over_ssh ports_over_relay built_in_relay'
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
export GOWORK=off

check_composition() {
  label=$1
  tags=$2
  expected=$3
  echo "Checking modules: $label"

  if [ -n "$tags" ]; then
    "$GO" test -race -tags "$tags" ./... > "$work/test.log" 2>&1 || { cat "$work/test.log"; exit 1; }
    "$GO" vet -tags "$tags" ./...
    "$GO" build -tags "$tags" -o "$work/zelport" ./cmd/zelport
    "$GO" list -deps -tags "$tags" ./cmd/zelport > "$work/deps"
  else
    "$GO" test -race ./... > "$work/test.log" 2>&1 || { cat "$work/test.log"; exit 1; }
    "$GO" vet ./...
    "$GO" build -o "$work/zelport" ./cmd/zelport
    "$GO" list -deps ./cmd/zelport > "$work/deps"
  fi

  for module in $modules; do
    case " $expected " in
      *" $module "*)
        if ! grep -Eq "^github.com/zelport/zelport/internal/modules/$module$" "$work/deps"; then
          echo "Expected module was not linked: $module"
          exit 1
        fi
        ;;
      *)
        if grep -Eq "^github.com/zelport/zelport/internal/modules/$module$" "$work/deps"; then
          echo "Excluded module was linked: $module"
          exit 1
        fi
        ;;
    esac
  done

  case " $expected " in
    *" ports_over_relay "*|*" built_in_relay "*) ;;
    *)
      if grep -Eq 'github.com/zelport/zelport-(sdk-go|relay)|github.com/(coder/websocket|flynn/noise|hashicorp/yamux)' "$work/deps"; then
        echo 'Excluded relay dependency was linked'
        exit 1
      fi
      ;;
  esac
  case " $expected " in
    *" built_in_relay "*) ;;
    *)
      if grep -Eq 'github.com/zelport/zelport-relay/relay$' "$work/deps"; then
        echo 'Excluded Built-in Relay server was linked'
        exit 1
      fi
      ;;
  esac
  case " $expected " in
    *" ports_over_relay "*) ;;
    *)
      if grep -Eq 'github.com/zelport/zelport-sdk-go/client$|github.com/hashicorp/yamux' "$work/deps"; then
        echo 'Excluded Ports over Relay client was linked'
        exit 1
      fi
      ;;
  esac

  "$work/zelport" --version
}

# No tags is the user-facing composition and includes every supported module.
check_composition standard '' "$modules"

# Tagged compositions are diagnostic checks of internal module boundaries.
check_composition core zelport_core ''
for module in $modules; do
  check_composition "$module" "zelport_core,$module" "$module"
done
check_composition relay-pair 'zelport_core,ports_over_relay,built_in_relay' 'ports_over_relay built_in_relay'
check_composition local-http 'zelport_core,local_ports,remote_http' 'local_ports remote_http'
check_composition http-relay 'zelport_core,remote_http,ports_over_relay' 'remote_http ports_over_relay'
check_composition ssh-relay 'zelport_core,ports_over_ssh,ports_over_relay' 'ports_over_ssh ports_over_relay'
check_composition explicit-all "zelport_core,$(printf '%s' "$modules" | tr ' ' ',')" "$modules"
