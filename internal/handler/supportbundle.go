package handler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SupportBundleHandler handles POST /support-bundle.
type SupportBundleHandler struct {
	sdkEndpoint string
}

func NewSupportBundleHandler(sdkEndpoint string) *SupportBundleHandler {
	return &SupportBundleHandler{sdkEndpoint: sdkEndpoint}
}

func (h *SupportBundleHandler) Generate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	bundlePath := filepath.Join(os.TempDir(), fmt.Sprintf("snip-bundle-%d.tar.gz", time.Now().UnixNano()))
	defer os.Remove(bundlePath)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "support-bundle",
		"--load-cluster-specs",
		"--output", bundlePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("support-bundle generation failed: %v\n%s", err, out)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Bundle generation failed: %v</span>`, err)
		return
	}

	data, err := os.ReadFile(bundlePath)
	if err != nil {
		log.Printf("reading bundle file: %v", err)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Failed to read bundle: %v</span>`, err)
		return
	}

	uploadURL := h.sdkEndpoint + "/api/v1/app/supportbundle"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		log.Printf("building upload request: %v", err)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Failed to build upload request: %v</span>`, err)
		return
	}
	req.Header.Set("Content-Type", "application/gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("uploading bundle to SDK: %v", err)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">Upload failed: %v</span>`, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("SDK returned HTTP %d on bundle upload", resp.StatusCode)
		fmt.Fprintf(w, `<span class="text-red-400 text-sm">SDK rejected bundle (HTTP %d)</span>`, resp.StatusCode)
		return
	}

	fmt.Fprint(w, `<span class="text-green-400 text-sm">Bundle uploaded — check Vendor Portal</span>`)
}
