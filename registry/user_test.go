package registry

import (
	"fmt"
	"testing"
)

var testClient *RegistryClient = NewClient()

func TestGetPackages(t *testing.T) {
	ctx := t.Context()
	testCases := []struct {
		name    string
		user    string
		wantErr bool
	}{{
		name:    "me <3",
		user:    "kmsec-uk",
		wantErr: false,
	}, {
		name:    "exists but no management of packages",
		user:    "topflite8",
		wantErr: false,
	}, {
		name:    "nonexistent",
		user:    "ldfvkposiiovxoopiaiupdoi",
		wantErr: true,
	}}
	for _, tc := range testCases {
		pkgs, err := testClient.GetPackagesForUser(ctx, tc.user)

		if err != nil {
			if !tc.wantErr {
				t.Errorf("TestGetPackages: %s: %v", tc.name, err)
				continue
			}
		}
		for pkg, perm := range pkgs {
			fmt.Printf("%s manages %s with %s permission\n", tc.user, pkg, perm)
		}
	}
}
