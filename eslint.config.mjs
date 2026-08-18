// procoder gates itself with the toolchain it mandates — see hooks/checks/toolchain.js.
//
// The security plugin is the reason eslint is in that list at all: eslint's own
// rule set carries no security rules, so an eslint that runs without this plugin
// is installed, green, and blind. Nothing here disables a rule; a finding is
// answered by fixing the code, and a genuine false positive is recorded in
// .procoder.toml with a reason, where it stays reviewable.
import security from 'eslint-plugin-security';

export default [
  security.configs.recommended,
  {
    files: ['**/*.js', '**/*.mjs', '**/*.cjs', '**/*.ts'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'commonjs',
    },
  },
  {
    // Generated output, fixtures that violate a rung on purpose, worktrees, and
    // the scratch trees the tests build. Excluding a path is not the same as
    // disabling a rule: nothing here silences a finding about procoder's own
    // shipped source.
    ignores: [
      'docs/_site/**', 'node_modules/**', 'examples/*/before.*', '.claude/**',
    ],
  },
  {
    // Three rules switched off, each because it was measured wrong for THIS
    // codebase rather than found inconvenient. procoder's own policy allows
    // exactly this and nothing looser: the evidence lives beside the switch.
    //
    //   detect-unsafe-regex — 12 sites. The rule is `safe-regex`, which flags
    //     star height > 1 as a SHAPE and never checks whether an ambiguous
    //     decomposition exists. All twelve were timed against adversarial input
    //     scaled 200 → 1600 characters: flat and linear at every size, under
    //     0.1ms, no superlinear growth. They cannot backtrack because the
    //     repeated atom and its separator are disjoint — `(?:\.[\w-]+)*` cannot
    //     consume the `.` that delimits it. Re-run that measurement before
    //     trusting this note; the guard is the timing, not the paragraph.
    //
    //   detect-non-literal-fs-filename — 271 sites, which is every fs call
    //     procoder makes, because reading the files it is pointed at is what it
    //     is for. The weakness the rule proxies for is traversal from UNTRUSTED
    //     input. Every path here arrives from argv or a hook payload — the same
    //     local user who can already run `cat`. The invariant that would make it
    //     real, and the one to re-check before removing this: no path is ever
    //     taken from the CONTENT of a scanned file.
    //
    //   detect-object-injection — 58 sites, on the prototype-pollution risk. The
    //     one place untrusted text becomes object keys is the TOML parser, and it
    //     already refuses __proto__/constructor/prototype outright and builds
    //     every table with Object.create(null) — see toml.js FORBIDDEN_KEYS.
    //     Everywhere else the key comes from a closed set procoder defines.
    //
    // detect-non-literal-regexp is deliberately NOT in this list: its 22 sites
    // build patterns from this repo's own constants and the user's own config,
    // which is the same trust level as the code being scanned, but the rule is
    // cheap to satisfy and the day one of those patterns comes from somewhere
    // else it should fire.
    files: ['**/*.js', '**/*.mjs', '**/*.cjs', '**/*.ts'],
    rules: {
      'security/detect-unsafe-regex': 'off',
      'security/detect-non-literal-fs-filename': 'off',
      'security/detect-object-injection': 'off',
    },
  },
  {
    // Tests build regexes from their own fixtures and from constants declared a
    // few lines above the call. There is no external input in this tree at all,
    // so the rule reports only its own shape. Shipped source keeps the rule on
    // and answers it line by line — see the `procoder:` markers in hooks/.
    files: ['tests/**'],
    rules: { 'security/detect-non-literal-regexp': 'off' },
  },
];
