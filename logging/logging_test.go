package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestMessagesUseConfiguredLevel(t *testing.T) {
	previousLevel := configuredLevel.Load()
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	t.Cleanup(func() {
		configuredLevel.Store(previousLevel)
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})

	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetFlags(0)

	if err := SetLevel("NOTICE"); err != nil {
		t.Fatal(err)
	}
	Debugf("hidden")
	Infof("also hidden")
	Noticef("visible notice")
	if message := output.String(); message != "NOTICE: visible notice\n" {
		t.Fatalf("notice level produced unexpected output: %q", message)
	}

	output.Reset()
	if err := SetLevel("debug"); err != nil {
		t.Fatal(err)
	}
	Debugf("directory %q took %s", "12/28", "11s")
	Infof("presigned %q", "data.tif")
	if message := output.String(); !strings.Contains(message, `DEBUG: directory "12/28" took 11s`) ||
		!strings.Contains(message, `INFO: presigned "data.tif"`) {
		t.Fatalf("unexpected debug output: %q", message)
	}

	if err := SetLevel("invalid"); err == nil {
		t.Fatal("expected an invalid log level error")
	}
	if err := SetLevel("WARNING"); err == nil {
		t.Fatal("expected an unsupported operator log level error")
	}
}

func TestTimedfUsesDurationLevel(t *testing.T) {
	previousLevel := configuredLevel.Load()
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	t.Cleanup(func() {
		configuredLevel.Store(previousLevel)
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})

	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetFlags(0)
	if err := SetLevel("DEBUG"); err != nil {
		t.Fatal(err)
	}

	Timedf(time.Second, "fast")
	Timedf(infoDurationThreshold+time.Millisecond, "slow")
	Timedf(noticeDurationThreshold+time.Millisecond, "very slow")

	message := output.String()
	for _, expected := range []string{
		"DEBUG: fast",
		"INFO: slow",
		"NOTICE: very slow",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("timed log does not contain %q: %q", expected, message)
		}
	}
}
