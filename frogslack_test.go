package frogslack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const (
	firstTip     = "IF YOU MEET A FROG ON THE ROAD, RMA IT FOR CREDIT TOWARDS ANOTHER FROG."
	testResponse = `{"tips":[
		{"tip":"IF YOU MEET A FROG ON THE ROAD, RMA IT FOR CREDIT TOWARDS ANOTHER FROG.","number":1015},
		{"tip":"DO NOT GAZE DIRECTLY AT FROG WITHOUT CLASS 3 EYE PROTECTION AND COMPLETION OF CERTIFICATE 254L3: \"DISASSEMBLING FROG FOR MAINTENANCE\".","number":2320}
	]}`
	signatureHeader = "X-Slack-Signature"
	timestampHeader = "X-Slack-Request-Timestamp"
	fakeSecret      = "e6b19c573432dcc6b075501d51b51bb8"
)

func signature(t *testing.T, secret, body string, timestamp int64) string {
	t.Helper()
	h := hmac.New(sha256.New, []byte(secret))
	val := fmt.Sprintf("v0:%s:%s", strconv.FormatInt(timestamp, 10), body)
	if _, err := h.Write([]byte(val)); err != nil {
		t.Fatalf("Error generating signature: %q", err)
	}
	return fmt.Sprintf("v0=%s", hex.EncodeToString(h.Sum(nil)))
}

func TestCroak(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(getTipsSuccess))
	defer ts.Close()
	apiUrl = ts.URL
	signingSecret = fakeSecret

	cases := []struct {
		body string
		want *Response
	}{
		{
			body: "token=barf",
			want: &Response{ResponseType: "in_channel", Text: firstTip},
		},
	}
	for _, c := range cases {
		now := time.Now()
		req := httptest.NewRequest("GET", "/", strings.NewReader(c.body))
		req.Header.Set(timestampHeader, strconv.FormatInt(now.Unix(), 10))
		req.Header.Set(signatureHeader, signature(t, fakeSecret, c.body, now.Unix()))
		w := httptest.NewRecorder()

		Croak(w, req)

		ct := w.Result().Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("w.Result().Header.Get(%q) = %q, wanted %q", "Content-Type", ct, "application/json")
			continue
		}
		d := json.NewDecoder(w.Result().Body)
		got := &Response{}
		if err := d.Decode(got); err != nil {
			t.Errorf("d.Decode(%v) = %q, wanted no error", got, err)
			continue
		}
		if diff := cmp.Diff(got, c.want); diff != "" {
			t.Errorf("Croak(%q) returned diff (got, want)\n %v", c.body, diff)
		}
	}
}

func TestCroakDeadBackend(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(getTipsSuccess))
	ts.Close()
	apiUrl = ts.URL
	want := &Response{ResponseType: RESPONSE_TYPE_EPHEMERAL, Text: "SORRY, NO TIPS RIGHT NOW. COME BACK."}

	now := time.Now()
	req := httptest.NewRequest("GET", "/", strings.NewReader("welp"))
	req.Header.Set(timestampHeader, strconv.FormatInt(now.Unix(), 10))
	req.Header.Set(signatureHeader, signature(t, fakeSecret, "welp", now.Unix()))
	w := httptest.NewRecorder()

	Croak(w, req)

	ct := w.Result().Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("w.Result().Header.Get(%q) = %q, wanted %q", "Content-Type", ct, "application/json")
	}
	d := json.NewDecoder(w.Result().Body)
	got := &Response{}
	if err := d.Decode(got); err != nil {
		t.Errorf("d.Decode(%v) = %q, wanted no error", got, err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Croak(%q) returned diff (got, want)\n %v", "welp", diff)
	}
}

func TestCroakNoResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()
	apiUrl = ts.URL
	want := &Response{ResponseType: RESPONSE_TYPE_EPHEMERAL, Text: "SORRY, NO TIPS RIGHT NOW. COME BACK."}

	now := time.Now()
	req := httptest.NewRequest("GET", "/", strings.NewReader("welp"))
	req.Header.Set(timestampHeader, strconv.FormatInt(now.Unix(), 10))
	req.Header.Set(signatureHeader, signature(t, fakeSecret, "welp", now.Unix()))
	w := httptest.NewRecorder()

	Croak(w, req)

	ct := w.Result().Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("w.Result().Header.Get(%q) = %q, wanted %q", "Content-Type", ct, "application/json")
	}
	d := json.NewDecoder(w.Result().Body)
	got := &Response{}
	if err := d.Decode(got); err != nil {
		t.Errorf("d.Decode(%v) = %q, wanted no error", got, err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Croak(%q) returned diff (got, want)\n %v", "welp", diff)
	}
}

func TestCroakAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	apiUrl = ts.URL
	want := &Response{ResponseType: RESPONSE_TYPE_EPHEMERAL, Text: "SORRY, NO TIPS RIGHT NOW. COME BACK."}

	now := time.Now()
	req := httptest.NewRequest("GET", "/", strings.NewReader("welp"))
	req.Header.Set(timestampHeader, strconv.FormatInt(now.Unix(), 10))
	req.Header.Set(signatureHeader, signature(t, fakeSecret, "welp", now.Unix()))
	w := httptest.NewRecorder()

	Croak(w, req)

	ct := w.Result().Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("w.Result().Header.Get(%q) = %q, wanted %q", "Content-Type", ct, "application/json")
	}
	d := json.NewDecoder(w.Result().Body)
	got := &Response{}
	if err := d.Decode(got); err != nil {
		t.Errorf("d.Decode(%v) = %q, wanted no error", got, err)
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Croak(%q) returned diff (got, want)\n %v", "welp", diff)
	}
}

func getTipsSuccess(w http.ResponseWriter, r *http.Request) {
	if _, err := fmt.Fprint(w, testResponse); err != nil {
		log.Fatalf("error in fake handler: %q", err)
	}
}
