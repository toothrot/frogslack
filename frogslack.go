package frogslack

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/golang/glog"
	"github.com/nlopes/slack"
)

const (
	RESPONSE_TYPE_IN_CHANNEL = "in_channel"
	RESPONSE_TYPE_EPHEMERAL  = "ephemeral"
)

var (
	apiUrl        = "https://frog.tips/api/1/tips"
	signingSecret = os.Getenv("SLACK_SIGNING_SECRET_SHH")
	clientId      = os.Getenv("SLACK_CLIENT_ID")
	clientSecret  = os.Getenv("SLACK_CLIENT_SECRET")
)

type Attachment struct {
	Text string `json:"text"`
}

type Response struct {
	ResponseType string       `json:"response_type"`
	Text         string       `json:"text"`
	Attachments  []Attachment `json:"attachments,omitempty"`
}

type Request struct {
}

type TipsResponse struct {
	Tips []Tip `json:"tips"`
}

type Tip struct {
	Tip    string `json:"tip"`
	Number int    `json:"number"`
}

func getTip(ctx context.Context) (Tip, error) {
	var tr TipsResponse
	var tip Tip
	req, err := http.NewRequest(http.MethodGet, apiUrl, nil)
	if err != nil {
		return tip, err
	}
	req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tip, err
	}
	d := json.NewDecoder(resp.Body)
	if err := d.Decode(&tr); err != nil {
		return tip, err
	}
	if len(tr.Tips) == 0 {
		return tip, errors.New("NOT ENOUGH TIPS")
	}
	return tr.Tips[0], err
}

func Croak(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.Errorf("I will not do this anymore %q", err)
		writeError(w, "SORRY, NO TIPS RIGHT NOW.")
		return
	}

	if err = verify(r.Header, body); err != nil {
		writeError(w, "SORRY, NO TIPS RIGHT NOW.")
		return
	}

	tip, err := getTip(ctx)
	if err != nil {
		glog.Errorf("getTip(%v) = _, %q", ctx, err)
		writeResponse(w, &Response{
			ResponseType: RESPONSE_TYPE_EPHEMERAL,
			Text:         "SORRY, NO TIPS RIGHT NOW. COME BACK.",
		})
		return
	}

	writeResponse(w, &Response{
		ResponseType: RESPONSE_TYPE_IN_CHANNEL,
		Text:         tip.Tip,
	})
}

func verify(h http.Header, body []byte) error {
	sv, err := slack.NewSecretsVerifier(h, signingSecret)
	if err != nil {
		glog.Errorf("Ohmy, no secrets? %q", err)
		return err
	}
	_, err = sv.Write(body)
	if err != nil {
		glog.Errorf("forget it %q", err)
		return err
	}
	if err = sv.Ensure(); err != nil {
		glog.Errorf("I can't do this anymore %q", err)
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, message string) {
	writeResponse(w, &Response{
		ResponseType: RESPONSE_TYPE_EPHEMERAL,
		Text:         message,
	})
}

func writeResponse(w http.ResponseWriter, resp *Response) {
	w.Header().Set("Content-Type", "application/json")
	e := json.NewEncoder(w)
	if err := e.Encode(resp); err != nil {
		glog.Errorf("e.Encode(%#v) = _, %q", resp, err)
	}
}

type accessResp struct {
	Ok    bool   `json:"ok"`
	AppId string `json:"app_id"`
	Error string `json:"error"`
	Team  struct {
		Id string `json:"id"`
	} `json:"team"`
}

func Hop(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	resp, err := http.PostForm("https://slack.com/api/oauth.v2.access", url.Values{
		"code":          []string{code},
		"client_id":     []string{clientId},
		"client_secret": []string{clientSecret},
		"redirect_uri": []string{""},
	})
	if err != nil {
		glog.Errorf("what hath god wrought: %v", err)
		return
	}
	defer resp.Body.Close()
	glog.Infof("hop'd: %v", resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("nobody %v", err)
		return
	}
	var ar accessResp
	if err := json.Unmarshal(body, &ar); err != nil {
		glog.Errorf("its not even jason: %v", err)
		return
	}
	glog.Infof("ok: %q, appId: %q, error: %q, Team: %+v", ar.Ok, ar.AppId, ar.Error, ar.Team)
	if !ar.Ok {
		w.Write([]byte("dang"))
		return
	}
	w.Write([]byte("it's fine."))
}
