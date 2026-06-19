// Merge two v2ray geoip.dat files into one, prefixing the second file's
// category codes so both namespaces coexist without collision.
//
// usage: mergegeoip <general.dat> <overlay.dat> <PREFIX> <out.dat>
//
//	general.dat — base (canonical category codes kept as-is, e.g. RU, PRIVATE)
//	overlay.dat — its entries get PREFIX prepended to country_code
//	PREFIX      — e.g. "ROSCOMVPN-" (codes are uppercase in the dat)
//	out.dat     — merged output
//
// Env:
//
//	IPV4_ONLY=1        — drop IPv6 CIDRs (client runs queryStrategy UseIPv4).
//	GEOIP_SRS_DIR=dir  — dump per-category IPv4 CIDR text (full set, for sing-box .srs).
//	USED_TAGS_FILE=f   — slim the OUTPUT .dat to the geoip: tags in allowlist f (INFRA-109,
//	                     Variant B). The CIDR dump (GEOIP_SRS_DIR) stays FULL — only the .dat
//	                     is slimmed. Every used geoip tag must exist in the merged set or build
//	                     fails (drift guard). Unset → full .dat (backward compatible).
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: mergegeoip <general.dat> <overlay.dat> <PREFIX> <out.dat>")
		os.Exit(2)
	}
	generalPath, overlayPath, prefix, outPath := os.Args[1], os.Args[2], os.Args[3], os.Args[4]

	general := load(generalPath)
	overlay := load(overlayPath)

	// Drop IPv6 CIDRs when IPV4_ONLY=1: the client runs queryStrategy UseIPv4,
	// so v6 ranges are dead weight — and dropping them keeps geoip.dat under
	// jsDelivr's 20 MB per-file limit.
	if os.Getenv("IPV4_ONLY") == "1" {
		keepIPv4(general)
		keepIPv4(overlay)
	}

	for _, e := range overlay.GetEntry() {
		e.CountryCode = prefix + e.GetCountryCode()
	}
	nGeneral, nOverlay := len(general.GetEntry()), len(overlay.GetEntry())
	general.Entry = append(general.Entry, overlay.GetEntry()...)
	fmt.Printf("merged: %d general + %d overlay (prefixed %q) = %d entries\n",
		nGeneral, nOverlay, prefix, len(general.GetEntry()))

	// Optional: dump per-category IPv4 CIDR text (one CIDR per line) for sing-box .srs.
	// Filename = lowercased country_code (e.g. ru.txt, roscomvpn-whitelist.txt).
	// NOTE: dump happens BEFORE the used-tags slim below, so .srs stay FULL.
	if dir := os.Getenv("GEOIP_SRS_DIR"); dir != "" {
		dumpCIDR(general, dir)
	}

	// Optional: slim the published .dat to a used-tag allowlist (Variant B). The .srs
	// dump above is already done with the full set, so nodes keep full coverage.
	if f := os.Getenv("USED_TAGS_FILE"); f != "" {
		slimToUsed(general, f)
	}

	out, err := proto.Marshal(general)
	if err != nil {
		fatal("marshal", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		fatal("write "+outPath, err)
	}
	fmt.Printf("wrote %s: %d entries, %d bytes\n", outPath, len(general.GetEntry()), len(out))
}

// dumpCIDR writes per-category IPv4 CIDR text files (one "a.b.c.d/p" per line) into dir.
func dumpCIDR(l *GeoIPList, dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal("mkdir "+dir, err)
	}
	for _, e := range l.GetEntry() {
		var b strings.Builder
		for _, c := range e.GetCidr() {
			ip := c.GetIp()
			if len(ip) != 4 {
				continue
			}
			fmt.Fprintf(&b, "%d.%d.%d.%d/%d\n", ip[0], ip[1], ip[2], ip[3], c.GetPrefix())
		}
		name := strings.ToLower(e.GetCountryCode())
		if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(b.String()), 0o644); err != nil {
			fatal("write cidr "+name, err)
		}
	}
}

// slimToUsed filters l.Entry in place down to the geoip: tags in the allowlist file.
// Guard: every requested geoip tag must exist in the merged set, else exit 1 (drift guard).
func slimToUsed(l *GeoIPList, usedPath string) {
	want := loadUsedGeoip(usedPath)
	if len(want) == 0 {
		fatal("used-tags", fmt.Errorf("no geoip: entries in %s", usedPath))
	}
	found := make(map[string]bool, len(want))
	kept := l.GetEntry()[:0]
	for _, e := range l.GetEntry() {
		code := strings.ToUpper(e.GetCountryCode())
		if want[code] {
			found[code] = true
			kept = append(kept, e)
		}
	}
	var missing []string
	for code := range want {
		if !found[code] {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(os.Stderr, "mergegeoip: %d used geoip tag(s) NOT in merged set:\n", len(missing))
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "  geoip:%s\n", strings.ToLower(m))
		}
		os.Exit(1)
	}
	l.Entry = kept
	fmt.Printf("slimmed geoip .dat: kept %d used categories\n", len(kept))
}

// loadUsedGeoip parses used-tags.txt → set of UPPERCASE geoip category codes.
func loadUsedGeoip(path string) map[string]bool {
	f, err := os.Open(path)
	if err != nil {
		fatal("open "+path, err)
	}
	defer f.Close()
	set := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if v, ok := strings.CutPrefix(line, "geoip:"); ok {
			set[strings.ToUpper(strings.TrimSpace(v))] = true
		}
	}
	if err := sc.Err(); err != nil {
		fatal("scan "+path, err)
	}
	return set
}

// keepIPv4 strips IPv6 CIDRs (ip length 16) from every entry, keeping IPv4 (length 4).
func keepIPv4(l *GeoIPList) {
	for _, e := range l.GetEntry() {
		kept := e.GetCidr()[:0]
		for _, c := range e.GetCidr() {
			if len(c.GetIp()) == 4 {
				kept = append(kept, c)
			}
		}
		e.Cidr = kept
	}
}

func load(path string) *GeoIPList {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read "+path, err)
	}
	var list GeoIPList
	if err := proto.Unmarshal(b, &list); err != nil {
		fatal("unmarshal "+path, err)
	}
	return &list
}

func fatal(ctx string, err error) {
	fmt.Fprintf(os.Stderr, "mergegeoip: %s: %v\n", ctx, err)
	os.Exit(1)
}
