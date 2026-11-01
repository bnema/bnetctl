package wine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Registry exports escape backslashes. Never execute Wine to inspect startup state.
func checkRunOnce(pfx string) error {
	var entries []string
	for _, name := range []string{"system.reg", "user.reg"} {
		f, err := os.Open(filepath.Join(pfx, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), 4*1024*1024)
		key := ""
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "[") {
				end := strings.Index(line, "]")
				key = ""
				if end >= 0 {
					key = strings.ReplaceAll(line[1:end], `\\`, `\`)
				}
				continue
			}
			lower := strings.ToLower(key)
			base := `software\microsoft\windows\currentversion\`
			wow := `software\wow6432node\microsoft\windows\currentversion\`
			dangerous := lower == base+"runonce" || lower == base+"runservicesonce" || lower == wow+"runonce" || lower == wow+"runservicesonce"
			if dangerous && (strings.HasPrefix(line, `"`) || strings.HasPrefix(line, "@=")) && strings.Contains(line, "=") {
				entries = append(entries, name+": ["+key+"] "+line)
			}
		}
		err = scanner.Err()
		_ = f.Close()
		if err != nil {
			return err
		}
	}
	if len(entries) > 0 {
		return fmt.Errorf("refusing prefix migration: wineboot -u -r still processes RunOnce/RunServicesOnce entries: %s", strings.Join(entries, "; "))
	}
	return nil
}
