package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"fitty/server/internal/intelligence"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This suite inserts chat and ledger fixtures directly. It never enqueues or
// claims analysis work, so it can run alongside the local development server.
func TestFavoritesIntegration(t *testing.T) {
	database, userA, userB := os.Getenv("FITTY_TEST_DATABASE_URL"), os.Getenv("FITTY_TEST_USER_A"), os.Getenv("FITTY_TEST_USER_B")
	if database == "" || userA == "" || userB == "" {
		t.Skip("Supply FITTY_TEST_DATABASE_URL and two temporary FITTY_TEST_USER_A/B accounts on local Supabase.")
	}
	endpoint, err := url.Parse(database)
	if err != nil || (endpoint.Hostname() != "127.0.0.1" && endpoint.Hostname() != "localhost") || endpoint.Port() != "54322" ||
		endpoint.User.Username() != "fitty_api" || !uuidPattern.MatchString(userA) || !uuidPattern.MatchString(userB) || userA == userB {
		t.Fatal("Favorites integration requires local fitty_api credentials and two distinct temporary users")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := pgxpool.New(ctx, database)
	if err != nil {
		t.Fatal("Could not initialize local test database connection")
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatal("Could not connect to local test database")
	}
	app := &App{DB: db, Auth: &Auth{URL: "https://auth.example.test", Key: "test-public-key", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		user := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		status, body := 401, `{}`
		if user == userA || user == userB {
			status, body = 200, `{"id":"`+user+`"}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}}
	for _, test := range []struct {
		name string
		run  func(*trackingHarness)
	}{
		{"ownership_versions_snapshot_and_source_deletion", testFavoriteLifecycle},
		{"parallel_retries_and_account_limit", testFavoriteConcurrency},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := &trackingHarness{t: t, ctx: ctx, app: app, router: New(nil, app), a: userA, b: userB}
			test.run(h)
		})
	}
	var jobs int
	if err := db.QueryRow(ctx, `select count(*) from fitty.analysis_jobs where user_id in ($1::uuid,$2::uuid)`, userA, userB).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("Favorites must never create analysis jobs: count=%d, error=%v", jobs, err)
	}
}

func favoriteFixture(h *trackingHarness, user, date string, value intelligence.Values) intelligence.Entry {
	h.t.Helper()
	var day, message int64
	err := h.app.DB.QueryRow(h.ctx, `insert into fitty.days(user_id,local_date,time_zone) values($1,$2::date,'Europe/Berlin')
 on conflict(user_id,local_date) do update set local_date=excluded.local_date returning id`, user, date).Scan(&day)
	if err != nil {
		h.t.Fatal(err)
	}
	err = h.app.DB.QueryRow(h.ctx, `insert into fitty.messages(user_id,day_id,client_id,role,content) values($1,$2,$3,'user','Offline favorite fixture') returning id`, user, day, trackingUUID(h.t)).Scan(&message)
	if err != nil {
		h.t.Fatal(err)
	}
	entry, err := scanEntry(h.app.DB.QueryRow(h.ctx, `insert into fitty.tracking_entries(user_id,day_id,source_message_id,updated_by_message_id,kind,label,amount,calories,protein_g,carbs_g,fat_g,duration_minutes,distance_km,source,notes)
 values($1,$2,$3,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) returning `+entryColumns,
		user, day, message, value.Kind, value.Label, value.Amount, value.Calories, value.ProteinG, value.CarbsG, value.FatG, value.DurationMinutes, value.DistanceKM, value.Source, value.Notes))
	if err != nil {
		h.t.Fatal(err)
	}
	return entry
}

func favoritesFor(h *trackingHarness, user string) []foodFavorite {
	h.t.Helper()
	var response struct {
		Favorites []foodFavorite `json:"favorites"`
	}
	h.request(user, "GET", "/v1/favorites", nil, 200, &response)
	if response.Favorites == nil {
		h.t.Fatal("favorites must be a JSON array, including an empty collection")
	}
	return response.Favorites
}

func testFavoriteLifecycle(h *trackingHarness) {
	const date = "2035-01-01"
	if len(favoritesFor(h, h.a)) != 0 || len(favoritesFor(h, h.b)) != 0 {
		h.t.Fatal("temporary accounts must begin without favorites")
	}
	values := trackingFood(456.78)
	values.Label, values.Amount, values.Notes = "Joghurt mit Haferflocken", "1 Schale, 250 g", "Mit Beeren"
	entry := favoriteFixture(h, h.a, date, values)
	path := "/v1/favorites/" + entry.ID
	input := favoriteChange{EntryVersion: entry.Version}
	h.request(h.b, "PUT", path, input, 404, nil)
	h.request(h.a, "PUT", "/v1/favorites/9223372036854775807", input, 404, nil)
	h.request(h.a, "PUT", path, favoriteChange{EntryVersion: entry.Version + 1}, 409, nil)
	activity := favoriteFixture(h, h.a, date, intelligence.Values{Kind: "activity", Label: "Walking Pad", DurationMinutes: trackingPtr(30.0), Source: "device"})
	h.request(h.a, "PUT", "/v1/favorites/"+activity.ID, favoriteChange{EntryVersion: activity.Version}, 400, nil)
	removed := favoriteFixture(h, h.a, date, trackingFood(100))
	h.request(h.a, "DELETE", "/v1/days/"+date+"/entries/"+removed.ID, entryChange{RequestID: trackingUUID(h.t), Version: removed.Version}, 200, nil)
	h.request(h.a, "PUT", "/v1/favorites/"+removed.ID, favoriteChange{EntryVersion: removed.Version}, 404, nil)
	before := h.summary(h.a, date)
	var response struct {
		Favorite foodFavorite `json:"favorite"`
	}
	h.request(h.a, "PUT", path, input, 200, &response)
	expected := foodFavorite{ID: entry.ID, Entry: entry.Values}
	if !reflect.DeepEqual(response.Favorite, expected) {
		h.t.Fatal("saved favorite lost its source portion, nutrition or provenance")
	}
	h.request(h.a, "PUT", path, input, 200, nil)
	h.request(h.b, "DELETE", path, nil, 200, nil)
	if got := favoritesFor(h, h.a); len(got) != 1 || !reflect.DeepEqual(got[0], expected) || len(favoritesFor(h, h.b)) != 0 {
		h.t.Fatal("favorite retry duplicated data or another account accessed it")
	}
	if got := h.summary(h.a, date); !reflect.DeepEqual(got, before) {
		h.t.Fatal("saving a favorite changed daily tracking data")
	}
	changed := entry.Values
	changed.Calories, changed.Source = trackingPtr(600.0), "user"
	entryPath := "/v1/days/" + date + "/entries/" + entry.ID
	h.request(h.a, "PUT", entryPath, entryChange{RequestID: trackingUUID(h.t), Version: entry.Version, Entry: &changed}, 200, nil)
	h.request(h.a, "PUT", path, input, 200, &response)
	if !reflect.DeepEqual(response.Favorite, expected) {
		h.t.Fatal("a changed source silently replaced the favorite snapshot")
	}
	h.request(h.a, "DELETE", path, nil, 200, nil)
	h.request(h.a, "DELETE", path, nil, 200, nil)
	h.request(h.a, "PUT", path, input, 409, nil)
	h.request(h.a, "PUT", path, favoriteChange{EntryVersion: entry.Version + 1}, 200, &response)
	if !reflect.DeepEqual(response.Favorite.Entry, changed) {
		h.t.Fatal("explicitly saving again must snapshot the current source")
	}
	h.request(h.a, "DELETE", entryPath, entryChange{RequestID: trackingUUID(h.t), Version: entry.Version + 1}, 200, nil)
	h.request(h.a, "PUT", path, input, 200, nil)
	if got := favoritesFor(h, h.a); len(got) != 1 || !reflect.DeepEqual(got[0].Entry, changed) {
		h.t.Fatal("soft-deleting an entry removed or changed its favorite")
	}
	device := values
	device.Source = "device"
	deviceEntry := favoriteFixture(h, h.a, date, device)
	h.request(h.a, "PUT", "/v1/favorites/"+deviceEntry.ID, favoriteChange{EntryVersion: 1}, 200, nil)
	if got := favoritesFor(h, h.a); len(got) != 2 || got[0].ID != deviceEntry.ID || got[0].Entry.Source != "device" {
		h.t.Fatal("favorites must preserve device provenance and list newest first")
	}
	h.request(h.a, "DELETE", "/v1/days/"+date, deletionRequest(h, date), 200, nil)
	h.request(h.a, "PUT", path, input, 200, nil)
	if got := favoritesFor(h, h.a); len(got) != 2 || !reflect.DeepEqual(got[1].Entry, changed) {
		h.t.Fatal("deleting the original day removed or changed a favorite")
	}
	for _, saved := range favoritesFor(h, h.a) {
		h.request(h.a, "DELETE", "/v1/favorites/"+saved.ID, nil, 200, nil)
	}
	h.request(h.a, "PUT", path, input, 404, nil)
	if len(favoritesFor(h, h.a)) != 0 {
		h.t.Fatal("removing favorites did not persist")
	}
}

func testFavoriteConcurrency(h *trackingHarness) {
	const date = "2035-02-01"
	first := favoriteFixture(h, h.a, date, trackingFood(500))
	second := favoriteFixture(h, h.a, date, trackingFood(600))
	call := func(id string) int {
		r := httptest.NewRequest("PUT", "/v1/favorites/"+id, strings.NewReader(`{"entry_version":1}`))
		r.Header.Set("Authorization", "Bearer "+h.a)
		w := httptest.NewRecorder()
		h.router.ServeHTTP(w, r)
		return w.Code
	}
	run := func(ids []string) []int {
		var wait sync.WaitGroup
		statuses := make([]int, len(ids))
		for i, id := range ids {
			wait.Go(func() { statuses[i] = call(id) })
		}
		wait.Wait()
		return statuses
	}
	for _, code := range run([]string{first.ID, first.ID, first.ID, first.ID}) {
		if code != 200 {
			h.t.Fatalf("parallel favorite retry returned %d", code)
		}
	}
	if len(favoritesFor(h, h.a)) != 1 {
		h.t.Fatal("parallel retries created duplicate favorites")
	}
	h.request(h.a, "DELETE", "/v1/favorites/"+first.ID, nil, 200, nil)
	// These snapshots represent favorites whose earlier source days are gone.
	raw, _ := json.Marshal(trackingFood(400))
	_, err := h.app.DB.Exec(h.ctx, `insert into fitty.food_favorites(user_id,entry_id,entry)
 select $1,9223372036854770000+i,$2::jsonb from generate_series(1,$3::integer) as fixture(i)`, h.a, raw, maxFoodFavorites-1)
	if err != nil {
		h.t.Fatal(err)
	}
	statuses := run([]string{first.ID, second.ID})
	if !((statuses[0] == 200 && statuses[1] == 409) || (statuses[0] == 409 && statuses[1] == 200)) {
		h.t.Fatalf("concurrent last-slot writes must have one success and one limit conflict: %v", statuses)
	}
	saved := favoritesFor(h, h.a)
	if len(saved) != maxFoodFavorites {
		h.t.Fatal("favorite limit was exceeded or returned an incomplete list")
	}
	winner, loser := first.ID, second.ID
	if statuses[1] == 200 {
		winner, loser = second.ID, first.ID
	}
	if saved[0].ID != winner || call(winner) != 200 {
		h.t.Fatal("newest favorite must be first and retries must succeed at the limit")
	}
	other := favoriteFixture(h, h.b, date, trackingFood(700))
	h.request(h.b, "PUT", "/v1/favorites/"+other.ID, favoriteChange{EntryVersion: 1}, 200, nil)
	if len(favoritesFor(h, h.b)) != 1 {
		h.t.Fatal("the favorite limit must apply independently to each account")
	}
	h.request(h.a, "DELETE", "/v1/favorites/"+winner, nil, 200, nil)
	if call(loser) != 200 || len(favoritesFor(h, h.a)) != maxFoodFavorites {
		h.t.Fatal("removing a favorite must free a slot")
	}
}
