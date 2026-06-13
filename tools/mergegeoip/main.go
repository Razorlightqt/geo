// Merge two v2ray geoip.dat files into one, prefixing the second file's
// category codes so both namespaces coexist without collision.
//
// usage: mergegeoip <general.dat> <overlay.dat> <PREFIX> <out.dat>
//
//	general.dat — base (canonical category codes kept as-is, e.g. RU, PRIVATE)
//	overlay.dat — its entries get PREFIX prepended to country_code
//	PREFIX      — e.g. "ROSCOMVPN-" (codes are uppercase in the dat)
//	out.dat     — merged output
package main

import (
	"fmt"
	"os"

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

	for _, e := range overlay.GetEntry() {
		e.CountryCode = prefix + e.GetCountryCode()
	}
	general.Entry = append(general.Entry, overlay.GetEntry()...)

	out, err := proto.Marshal(general)
	if err != nil {
		fatal("marshal", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		fatal("write "+outPath, err)
	}
	fmt.Printf("merged: %d general + %d overlay (prefixed %q) = %d entries -> %s (%d bytes)\n",
		len(general.GetEntry())-len(overlay.GetEntry()), len(overlay.GetEntry()), prefix,
		len(general.GetEntry()), outPath, len(out))
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
