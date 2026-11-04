package mozjs

import (
	"runtime"
	"testing"
)

func TestAndroidSpiderMonkeyLifecycle(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for range 2 {
		SetCamouLocale("en", "US", "en-US")
		cx, err := NewContext(Options{HeapBytes: 64 << 20, Locale: "en-US"})
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer cx.Close()
			global, err := cx.NewGlobal(0, "UTC")
			if err != nil {
				t.Fatal(err)
			}
			value, err := cx.Evaluate(global, "6 * 7", "android-smoke.js", 1)
			if err != nil || value.Kind != KindNumber || value.Number != 42 {
				t.Fatalf("JavaScript evaluation: %+v, %v", value, err)
			}
			value, err = cx.Evaluate(global, "new Intl.NumberFormat('en-US').format(1234)", "android-smoke.js", 1)
			if err != nil || value.Kind != KindString || value.String != "1,234" {
				t.Fatalf("Intl evaluation: %+v, %v", value, err)
			}
			if _, err := cx.Evaluate(global, "globalThis.probe = 0; Promise.resolve(42).then(v => globalThis.probe = v)", "android-smoke.js", 1); err != nil {
				t.Fatal(err)
			}
			cx.RunJobs()
			value, err = cx.Evaluate(global, "probe", "android-smoke.js", 1)
			if err != nil || value.Number != 42 {
				t.Fatalf("Promise jobs: %+v, %v", value, err)
			}
			cx.GC()
		}()
	}
}
