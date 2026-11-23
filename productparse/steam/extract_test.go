package steam

import (
	"encoding/json"
	"testing"
)

const sampleAppDetailsJSON = `{
  "2322010": {
    "success": true,
    "data": {
      "name": "God of War Ragnarök",
      "is_free": false,
      "header_image": "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/2322010/header.jpg",
      "price_overview": {
        "currency": "USD",
        "initial": 5999,
        "final": 5999
      }
    }
  }
}`

func TestValidateProductURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{
			"https://store.steampowered.com/app/2322010/God_of_War_Ragnaryok/",
			false,
		},
		{
			"https://store.steampowered.com/app/2322010/",
			false,
		},
		{
			"https://store.steampowered.com/wishlist/id/foo/",
			true,
		},
	}

	for _, tt := range tests {
		err := validateProductURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Fatalf("validateProductURL(%q) err=%v wantErr=%v", tt.url, err, tt.wantErr)
		}
	}
}

func TestExtractAppID(t *testing.T) {
	if got := extractAppID("/app/2322010/slug/"); got != "2322010" {
		t.Fatalf("got %q", got)
	}
}

func TestSnapshotFromAppDetails(t *testing.T) {
	var payload appDetailsResponse
	if err := json.Unmarshal([]byte(sampleAppDetailsJSON), &payload); err != nil {
		t.Fatal(err)
	}

	snapshot, err := snapshotFromAppDetails(*payload["2322010"].Data)
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.Title != "God of War Ragnarök" {
		t.Fatalf("title=%q", snapshot.Title)
	}
	if snapshot.Price != 59.99 {
		t.Fatalf("price=%v", snapshot.Price)
	}
	if snapshot.ImageURL == "" {
		t.Fatal("expected image")
	}
}

func TestSnapshotFromFreeApp(t *testing.T) {
	snapshot, err := snapshotFromAppDetails(appDetailsData{
		Name:        "Free Game",
		IsFree:      true,
		HeaderImage: "https://cdn.example/header.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Price != 0 {
		t.Fatalf("price=%v", snapshot.Price)
	}
}
