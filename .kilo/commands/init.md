---
description: Install the formatters this repository needs, with every command visible before it runs.
---

The user invoked /procoder:init.

1. Run:

       procoder init

   It surveys the repository and prints one install command per missing
   formatter, chosen for this machine's package managers.

2. Execute each printed command yourself, one at a time, so the user sees
   exactly what is being installed and how. If a command fails, show the error
   and stop — do not improvise a different installer; the printed command is
   the supported path.

3. For any line saying "install by hand", relay it to the user — procoder
   found no package manager it knows on this machine.

4. Finish by running:

       procoder doctor

   and confirm every gap is closed. A tool is installed when doctor says ok —
   an installer exiting 0 is not the proof; doctor is.
