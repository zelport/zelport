package privatefile

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtectedDACLBeforeWriteAndAfterReplace(t *testing.T) {
	dir := t.TempDir()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	f, err := CreateTemp(dir)
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	defer os.Remove(name)
	check := func(path string) {
		t.Helper()
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		acl, _, err := sd.DACL()
		if err != nil || acl == nil || acl.AceCount != 2 {
			t.Fatal("unexpected ACL", acl, err)
		}
		s := sd.String()
		if !strings.Contains(s, "D:P") || !strings.Contains(s, user.User.Sid.String()) || !strings.Contains(s, ";;;SY)") {
			t.Fatal(s)
		}
	}
	check(name)
	if _, err = f.WriteString("secret"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "config.json")
	os.WriteFile(target, []byte("old"), 0600)
	if err = Replace(name, target); err != nil {
		t.Fatal(err)
	}
	check(target)
}
