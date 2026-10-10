package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const rainyBCSWeather = "[name]\nBCS rain\n[fog]\n3500\n0.9\n[wind]\n170\n4.5\n[temp]\n12\n11\n[press]\n1002\n[clouds]\nOvercast 1\n800\n[precip]\n1\n96\n0\n0\n0\n[groundwet]\n200\n0\n0\n"

func TestBCSWeatherPreservesRainInsteadOfPhysicalDefault(t *testing.T) {
	for _, encoding := range []string{"utf8", "le", "be"} {
		weather, err := bridgeWeatherFromOWT(encodedLog(rainyBCSWeather, encoding))
		if err != nil || !validBridgeWeather(weather) || !strings.Contains(weather, "pt=1;pi=96.000") || !strings.Contains(weather, "c=4;cb=800") || !strings.Contains(weather, "wet=0.7843") {
			t.Fatal(encoding, weather, err)
		}
	}
	snow, err := bridgeWeatherFromOWT([]byte(strings.Replace(rainyBCSWeather, "[precip]\n1", "[precip]\n2", 1) + "[snow]\n[snowonroad]\n"))
	if err != nil || !strings.Contains(snow, "pt=2") || !strings.HasSuffix(snow, "snow=1;snowroad=1") {
		t.Fatal(snow, err)
	}
}

func TestWeatherSnapshotBoundToMapDateAndRecentStableFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "maps", "Sample")
	_ = os.MkdirAll(dir, 0700)
	situation := filepath.Join(dir, "laststn.osn")
	_ = os.WriteFile(situation, []byte("[map]\nmaps\\Sample\\global.cfg\n[time]\n2026\n280\n15\n30\n0\n"), 0600)
	_ = os.WriteFile(situation+".owt", encodedLog(rainyBCSWeather, "le"), 0600)
	weather, source, err := readBCSWeather(root, "maps/Sample/global.cfg", "2026-10-07", time.Now())
	if err != nil || source != situation+".owt" || !strings.Contains(weather, "pt=1") {
		t.Fatal(weather, source, err)
	}
	_ = os.WriteFile(situation, []byte("[map]\n"+filepath.Join(dir, "global.cfg")+"\n[time]\n2026\n280\n15\n30\n0\n"), 0600)
	if _, _, err = readBCSWeather(root, "maps/Sample/global.cfg", "2026-10-07", time.Now()); err != nil {
		t.Fatal("absolute BBS map path rejected", err)
	}
	if _, _, err = readBCSWeather(root, "maps/Sample/global.cfg", "2026-10-08", time.Now()); err == nil {
		t.Fatal("previous day weather reused")
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(situation+".owt", old, old)
	if _, _, err = readBCSWeather(root, "maps/Sample/global.cfg", "2026-10-07", time.Now()); err == nil {
		t.Fatal("stale weather reused")
	}
	_ = os.Chtimes(situation+".owt", time.Now(), time.Now())
	_ = os.WriteFile(situation, []byte("[map]\nmaps/Other/global.cfg\n[time]\n2026\n280\n"), 0600)
	if _, _, err = readBCSWeather(root, "maps/Sample/global.cfg", "2026-10-07", time.Now()); err == nil {
		t.Fatal("other map weather reused")
	}
}

func TestWeatherDemandRejectsFilesCommandsAndInvalidValues(t *testing.T) {
	valid, err := bridgeWeatherFromOWT([]byte(rainyBCSWeather))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"Weather/rain.owt", "custom:pt=1", valid + ";pt=2", strings.Replace(valid, "pt=1", "pt=3", 1), strings.Replace(valid, "pi=96.000", "pi=NaN", 1), strings.Replace(valid, "pi=96.000", "pi=96\nadmin_password = exposed", 1)} {
		if validBridgeWeather(bad) {
			t.Fatal("invalid network weather accepted", bad)
		}
	}
	for _, bad := range []string{rainyBCSWeather[:strings.Index(rainyBCSWeather, "[precip]")], strings.Replace(rainyBCSWeather, "\n96\n", "\nNaN\n", 1)} {
		if _, err = bridgeWeatherFromOWT([]byte(bad)); err == nil {
			t.Fatal("partial/invalid OWT accepted")
		}
	}
}
