package device

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func Detect(requested []int) ([]int, error) {
	if requested != nil {
		return requested, nil
	}
	cmd := exec.Command("nvidia-smi", "-L")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("HFT device detection failed; set HFT_DEVICES explicitly or run with NVIDIA runtime")
	}
	var devices []int
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "GPU ") {
			continue
		}
		rest := strings.TrimPrefix(line, "GPU ")
		indexText := rest
		if colon := strings.Index(rest, ":"); colon >= 0 {
			indexText = rest[:colon]
		}
		index, err := strconv.Atoi(strings.TrimSpace(indexText))
		if err == nil {
			devices = append(devices, index)
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("HFT device detection found no GPUs; set HFT_DEVICES explicitly if this is expected")
	}
	return devices, nil
}
