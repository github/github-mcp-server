package cpu_test

import (
	"testing"

	"github.com/segmentio/asm/cpu/arm64"
	"github.com/segmentio/asm/cpu/cpuid"
	"github.com/segmentio/asm/cpu/x86"
)

var x86Tests = map[string]cpuid.Feature{
	"SSE":                cpuid.Feature(x86.SSE),
	"SSE2":               cpuid.Feature(x86.SSE2),
	"SSE3":               cpuid.Feature(x86.SSE3),
	"SSE41":              cpuid.Feature(x86.SSE41),
	"SSE42":              cpuid.Feature(x86.SSE42),
	"SSE4A":              cpuid.Feature(x86.SSE4A),
	"SSSE3":              cpuid.Feature(x86.SSSE3),
	"AVX":                cpuid.Feature(x86.AVX),
	"AVX2":               cpuid.Feature(x86.AVX2),
	"AVX512BF16":         cpuid.Feature(x86.AVX512BF16),
	"AVX512BITALG":       cpuid.Feature(x86.AVX512BITALG),
	"AVX512BW":           cpuid.Feature(x86.AVX512BW),
	"AVX512CD":           cpuid.Feature(x86.AVX512CD),
	"AVX512DQ":           cpuid.Feature(x86.AVX512DQ),
	"AVX512ER":           cpuid.Feature(x86.AVX512ER),
	"AVX512F":            cpuid.Feature(x86.AVX512F),
	"AVX512IFMA":         cpuid.Feature(x86.AVX512IFMA),
	"AVX512PF":           cpuid.Feature(x86.AVX512PF),
	"AVX512VBMI":         cpuid.Feature(x86.AVX512VBMI),
	"AVX512VBMI2":        cpuid.Feature(x86.AVX512VBMI2),
	"AVX512VL":           cpuid.Feature(x86.AVX512VL),
	"AVX512VNNI":         cpuid.Feature(x86.AVX512VNNI),
	"AVX512VP2INTERSECT": cpuid.Feature(x86.AVX512VP2INTERSECT),
	"AVX512VPOPCNTDQ":    cpuid.Feature(x86.AVX512VPOPCNTDQ),
}

var arm64Tests = map[string]cpuid.Feature{
	"ASIMD":    cpuid.Feature(arm64.ASIMD),
	"ASIMDDP":  cpuid.Feature(arm64.ASIMDDP),
	"ASIMDHP":  cpuid.Feature(arm64.ASIMDHP),
	"ASIMDRDM": cpuid.Feature(arm64.ASIMDRDM),
}

func TestCPU(t *testing.T) {
	for _, test := range []struct {
		arch string
		feat map[string]cpuid.Feature
	}{
		{arch: "x86", feat: x86Tests},
		{arch: "arm64", feat: arm64Tests},
	} {
		t.Run("none", func(t *testing.T) {
			c := cpuid.CPU(cpuid.None)

			for name, feature := range test.feat {
				t.Run(name, func(t *testing.T) {
					if c.Has(feature) {
						t.Error("cpuid.None must not have any features enabled")
					}
				})
			}
		})

		t.Run("all", func(t *testing.T) {
			c := cpuid.CPU(cpuid.All)

			for name, feature := range test.feat {
				t.Run(name, func(t *testing.T) {
					if !c.Has(feature) {
						t.Errorf("missing a feature that should have been enabled by cpuid.All")
					}
				})
			}
		})

		t.Run("single", func(t *testing.T) {
			for name, feature := range test.feat {
				t.Run(name, func(t *testing.T) {
					c := cpuid.CPU(0)
					c.Set(feature, true)

					for n, f := range test.feat {
						if n == name {
							if !c.Has(f) {
								t.Errorf("expected feature not set on CPU: %s", n)
							}
						} else {
							if c.Has(f) {
								t.Errorf("unexpected feature set on CPU: %s", n)
							}
						}
					}
				})
			}
		})
	}
}
