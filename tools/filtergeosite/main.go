// filtergeosite slims a compiled v2fly geosite .dat down to a used-tag allowlist.
//
// usage: filtergeosite <in.dat> <used-tags.txt> <out.dat>
//
//	in.dat        — full compiled geosite.dat (transient; never published)
//	used-tags.txt — flat allowlist (geoip:/geosite: prefixed, # comments ok); only geosite: lines used here
//	out.dat       — slim geosite.dat containing ONLY the used categories
//
// Why wire-level (protowire) and not a generated .pb.go: the compiled GeoSiteList is
// self-contained — v2fly `include:` directives are already flattened into each category's
// inline domain list at compile time. So filtering by top-level category name is safe and
// every kept GeoSite entry is copied byte-for-byte (we never touch its domain list). This
// avoids vendoring a fragile geosite.pb.go and can't corrupt entries it keeps.
//
// GeoSiteList wire layout (proto3):
//
//	GeoSiteList: field 1 (repeated, LEN) = GeoSite
//	GeoSite:     field 1 (LEN/string)    = country_code   (uppercase category name)
//	             field 2 (repeated, LEN) = domain
//
// Guard: every geosite: tag in the allowlist MUST exist in in.dat, else exit 1 (catches
// typos and upstream renames — the only drift protection in Variant B).
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: filtergeosite <in.dat> <used-tags.txt> <out.dat>")
		os.Exit(2)
	}
	inPath, usedPath, outPath := os.Args[1], os.Args[2], os.Args[3]

	data, err := os.ReadFile(inPath)
	if err != nil {
		fatal("read "+inPath, err)
	}
	want := loadUsedGeosite(usedPath) // set of UPPERCASE category codes
	if len(want) == 0 {
		fatal("used-tags", fmt.Errorf("no geosite: entries in %s", usedPath))
	}

	var out []byte
	found := make(map[string]bool, len(want))
	kept, total := 0, 0

	b := data
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			fatal("parse", protowire.ParseError(n))
		}
		b = b[n:]
		if num == 1 && typ == protowire.BytesType {
			entry, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				fatal("parse entry", protowire.ParseError(n2))
			}
			b = b[n2:]
			total++
			code := strings.ToUpper(peekCountryCode(entry))
			if want[code] {
				found[code] = true
				out = protowire.AppendTag(out, 1, protowire.BytesType)
				out = protowire.AppendBytes(out, entry) // verbatim — domains untouched
				kept++
			}
		} else {
			// No other top-level fields exist in a GeoSiteList; skip defensively.
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				fatal("parse skip", protowire.ParseError(n2))
			}
			b = b[n2:]
		}
	}

	// Guard: fail loud on any requested tag missing from the base.
	var missing []string
	for code := range want {
		if !found[code] {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(os.Stderr, "filtergeosite: %d used geosite tag(s) NOT in base (%s):\n", len(missing), inPath)
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "  geosite:%s\n", strings.ToLower(m))
		}
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		fatal("write "+outPath, err)
	}
	fmt.Printf("filtergeosite: kept %d/%d categories (%d bytes) -> %s\n", kept, total, len(out), outPath)
}

// peekCountryCode reads GeoSite.country_code (field 1, string) without decoding the rest.
func peekCountryCode(entry []byte) string {
	b := entry
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return ""
		}
		b = b[n:]
		if num == 1 && typ == protowire.BytesType {
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return ""
			}
			return string(v)
		}
		n2 := protowire.ConsumeFieldValue(num, typ, b)
		if n2 < 0 {
			return ""
		}
		b = b[n2:]
	}
	return ""
}

// loadUsedGeosite parses used-tags.txt and returns the set of UPPERCASE geosite category codes.
func loadUsedGeosite(path string) map[string]bool {
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
		if v, ok := strings.CutPrefix(line, "geosite:"); ok {
			set[strings.ToUpper(strings.TrimSpace(v))] = true
		}
	}
	if err := sc.Err(); err != nil {
		fatal("scan "+path, err)
	}
	return set
}

func fatal(ctx string, err error) {
	fmt.Fprintf(os.Stderr, "filtergeosite: %s: %v\n", ctx, err)
	os.Exit(1)
}
