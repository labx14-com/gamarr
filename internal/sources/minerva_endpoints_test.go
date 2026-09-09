package sources

import "testing"

func TestMinervaEndpointDefaultsAndEnvOverrides(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if r.Minerva.APIURL != "https://minerva-archive.org/v1/api/" {
		t.Fatalf("default api url = %q", r.Minerva.APIURL)
	}
	if r.Minerva.TorrentsURL != "https://cdn.minerva-archive.org/torrents/" {
		t.Fatalf("default torrents url = %q", r.Minerva.TorrentsURL)
	}

	env := map[string]string{
		"MINERVA_API_URL":      "https://example.test/v1/api/",
		"MINERVA_TORRENTS_URL": "https://cdn.example.test/torrents/",
	}
	r.ApplyEnvOverrides(func(key string) string { return env[key] })
	if r.Minerva.APIURL != env["MINERVA_API_URL"] || r.Minerva.TorrentsURL != env["MINERVA_TORRENTS_URL"] {
		t.Fatalf("overrides = api:%q torrents:%q", r.Minerva.APIURL, r.Minerva.TorrentsURL)
	}
}

func TestLegacyMinervaAssetsURLMapsToTorrentURL(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	r.ApplyEnvOverrides(func(key string) string {
		if key == "MINERVA_ASSETS_URL" {
			return "https://legacy.example.test/torrents/"
		}
		return ""
	})
	if r.Minerva.TorrentsURL != "https://legacy.example.test/torrents/" {
		t.Fatalf("legacy torrents url = %q", r.Minerva.TorrentsURL)
	}
}
