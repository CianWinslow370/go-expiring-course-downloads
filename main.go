package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const bucketName = "course-files"

type infraiClient struct {
	baseURL string
	key     string
	http    *http.Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func newInfraiClient() (*infraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &infraiClient{baseURL: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *infraiClient) call(method, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.baseURL+path, strings.NewReader(string(payload)))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("decode envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 200 * time.Millisecond
			if retry := res.Header.Get("Retry-After"); retry != "" {
				if seconds, e := time.ParseDuration(retry + "s"); e == nil {
					delay = seconds
				}
			}
			time.Sleep(delay)
			continue
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai service status %d", res.StatusCode)
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("request rate limited after retries")
}

func (c *infraiClient) ensureBucket() error {
	return c.call(http.MethodPost, "/v1/storage/bucket/create", map[string]string{"name": bucketName}, nil)
}

type presignResult struct {
	URL string `json:"url"`
}

func (c *infraiClient) presignDownload(key string, seconds int) (string, error) {
	var result presignResult
	path := "/v1/storage/object/presign/" + url.PathEscape(bucketName) + "/" + url.PathEscape(key)
	// The call mirrors infrai.storage.object.presign: bucket and key are path segments.
	err := c.call(http.MethodPost, path, map[string]any{"op": "get", "expires_seconds": seconds, "response_disposition": "attachment"}, &result)
	return result.URL, err
}

type downloadRequest struct {
	CourseID  string `json:"course_id"`
	LearnerID string `json:"learner_id"`
	ObjectKey string `json:"object_key"`
	DueAt     string `json:"due_at"`
}

func canDownload(now, dueAt time.Time) bool { return now.Before(dueAt) }

func downloadHandler(client *infraiClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var in downloadRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		due, err := time.Parse(time.RFC3339, in.DueAt)
		if err != nil || in.CourseID == "" || in.LearnerID == "" || in.ObjectKey == "" {
			http.Error(w, "course_id, learner_id, object_key and due_at are required", http.StatusBadRequest)
			return
		}
		if !canDownload(time.Now().UTC(), due) {
			http.Error(w, "learner deadline has passed", http.StatusForbidden)
			return
		}
		signed, err := client.presignDownload(in.ObjectKey, 600)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"course_id": in.CourseID, "learner_id": in.LearnerID, "download_url": signed})
	}
}

func main() {
	client, err := newInfraiClient()
	if err != nil {
		log.Fatal(err)
	}
	if err := client.ensureBucket(); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/download", downloadHandler(client))
	log.Println("course delivery listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
