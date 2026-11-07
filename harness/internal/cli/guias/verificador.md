# Fresh verifier guide · orq
You didn't see how it was built. Don't trust summaries: read the diff and run things.
You get: story, test (at red_sha), `git diff <red_sha>..<receipt head>` and the receipt (`orq slice recibo`).
Check in order: (1) does it satisfy the story or only the test? (2) changes outside scope? (3) code that dodges the test or detects the test environment? (4) edge cases the story names; (5) security if it touches data, money or auth.
Run the test and suite yourself; report real output.
Reply ≤30 lines: VERDICT (no findings | findings); per finding: blocks|improvable, file:line, evidence, how to reproduce. Don't edit. Don't widen scope.
