package update

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0-beta.2", "0.1.0-beta.2", 0},
		{"0.1.0-beta.2", "0.1.0-test.8", 1}, // beta > test (Webor prerelease order)
		{"0.1.0-beta.3", "0.1.0-test.8", 1},
		{"0.1.0", "0.1.0-beta.1", 1},
		{"0.1.0-beta.1", "0.1.0", -1},
		{"0.1.0-beta.2", "0.1.0-beta.1", 1},
		{"0.2.0", "0.1.9", 1},
		{"v0.1.0", "0.1.0", 0},
		{"0.1.0-test.8", "0.1.0-test.9", -1},
	}
	for _, tc := range cases {
		got := Compare(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("Compare(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestInferChannelFromVersion(t *testing.T) {
	if InferChannelFromVersion("0.1.0-beta.3") != "beta" {
		t.Fatal("want beta")
	}
	if InferChannelFromVersion("0.1.0-test.8") != "stable" {
		t.Fatal("want stable")
	}
	if InferChannelFromVersion("1.0.0") != "stable" {
		t.Fatal("want stable")
	}
}

func TestValidateChannel(t *testing.T) {
	if err := ValidateChannel("beta"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChannel("nightly"); err == nil {
		t.Fatal("expected error")
	}
}
