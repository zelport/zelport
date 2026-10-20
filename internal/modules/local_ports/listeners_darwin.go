//go:build darwin

package local_ports

func newListenerScanner() listenerScanner {
	return newLsofScanner()
}
