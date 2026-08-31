// The one rule that turns a Claude Code command into a command for a host
// without a plugin root.
//
// commands/*.md is written once, for Claude Code, and says
// "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh". Hosts that set no such variable
// need the same sentence with the launcher named instead — OpenCode and Kilo
// name the binary on PATH, pi names the launcher beside the installed package.
//
// It lives here rather than inside internal/portability/portability_test.go
// because the rule has two callers that run in different languages: the Go
// parity check applies it to prove the OpenCode twins still match commands/,
// and the pi adapter applies it at load. In one file, a drift between them is a
// red test; in two, it is a command that runs the wrong binary, which nothing
// notices.
//
// `launcher` is the string that stands for procoder in the host: `procoder`
// where the host resolves it from PATH, a quoted absolute path where the caller
// knows where its own copy lives.

// Every Claude-only launcher form, longest first, replaced in ONE pass.
//
// One pass is not a style choice. The plugin-root form ends in `launcher.sh`,
// so a sequence of separate replacements re-scans the text it just inserted:
// asking for a launcher whose own name contains `launcher.sh` produced
// `"/opt/pkg/hooks/"/opt/pkg/hooks/launcher.sh""`. Invisible while the other
// caller substitutes `procoder`, which cannot contain it, and found the moment
// this repository asked for an absolute path.
const FORMS =
  /"\$\{CLAUDE_PLUGIN_ROOT\}\/hooks\/launcher\.sh"|launcher\.sh |launcher\.sh/g;

export function hostCommandText(body, launcher) {
  // CRLF levelled first: a Windows checkout rewrites line endings, and the
  // multi-line phrase below would otherwise never match there.
  const text = String(body).replaceAll("\r\n", "\n");

  // A trailing space belongs to the sentence, not to the match, so it is put
  // back rather than eaten by the alternative that matched it.
  const swapped = text.replace(FORMS, (form) =>
    form.endsWith(" ") ? `${launcher} ` : launcher,
  );

  // The two sentences that explain the launcher are only true of a
  // PATH-resolved one, and they are pinned byte for byte by the OpenCode twin
  // parity check. A host that names its own launcher needs no such sentence:
  // "The launcher is: /opt/procoder/hooks/launcher.sh" already says the true
  // thing, and rewriting it to mention PATH would trade a clear instruction for
  // a false one.
  if (launcher.includes("/")) return swapped;

  return swapped
    .replaceAll(
      "The launcher is: procoder",
      "The command below is the `procoder` binary on PATH.",
    )
    .replaceAll(
      "The launcher for every procoder command below is:\nprocoder",
      "Every procoder command below is the `procoder` binary on PATH.",
    );
}
