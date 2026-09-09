package check

import "testing"

func TestCheckMathDoesNotHideRealDiagnostics(t *testing.T) {
	t.Parallel()
	source := "---\ntitle: Math\n---\n\n# Math\n\n## Formula\n\n$$\ndp[r][c]\n=\n\\min(dp[r-1][c],dp[r][c-1])\n[text][missing]\n[^unknown]\n<img src=\"absent.png\">\n$$\n\n### After\n\n$dp[i][j]$ and [real][missing]\n\n# Real second H1\n\n### Real skip\n\n| A | B | C |\n|---|---|---|\n| OR | `>=1 <2 || >=3 <4` | Text |\n"
	expectDiagnostics(t, "math and real errors", source, Options{}, []string{
		"guide.md:20:16 error reference.undefined",
		"guide.md:22:1 warning document.multiple-h1",
		"guide.md:24:1 warning heading.level-skip",
		"guide.md:28:1 error table.column-mismatch",
	})
}
