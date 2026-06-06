package s3

import "testing"

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"缺 AK/SK", Config{Bucket: "b"}, true},
		{"缺 Bucket", Config{AccessKeyID: "ak", SecretAccessKey: "sk"}, true},
		{"齐备", Config{AccessKeyID: "ak", SecretAccessKey: "sk", Bucket: "b"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("New() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestNewDefaultsRegion(t *testing.T) {
	c, err := New(Config{AccessKeyID: "ak", SecretAccessKey: "sk", Bucket: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.Region != "us-east-1" {
		t.Fatalf("Region 默认值=%q, 期望 us-east-1", c.cfg.Region)
	}
}

func TestPublicURL(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		key  string
		want string
	}{
		{
			name: "优先 PublicBaseURL",
			cfg:  Config{AccessKeyID: "ak", SecretAccessKey: "sk", Bucket: "b", PublicBaseURL: "https://cdn.example.com/"},
			key:  "/a/b.png",
			want: "https://cdn.example.com/a/b.png",
		},
		{
			name: "回退端点+桶",
			cfg:  Config{AccessKeyID: "ak", SecretAccessKey: "sk", Bucket: "mybucket", Endpoint: "http://127.0.0.1:9000"},
			key:  "x.txt",
			want: "http://127.0.0.1:9000/mybucket/x.txt",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := New(tc.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.PublicURL(tc.key); got != tc.want {
				t.Fatalf("PublicURL()=%q, 期望 %q", got, tc.want)
			}
		})
	}
}
