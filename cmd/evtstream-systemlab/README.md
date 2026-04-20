# evtstream-systemlab

`evtstream-systemlab` is a separate app used to explain, exercise, and validate the public API boundaries of `pkg/evtstream`.

Goals:

- keep the playground separate from substrate code,
- consume only public `evtstream` APIs,
- provide narrated labs for each implementation phase,
- make debugging and onboarding easier.

## Boundary contract

Systemlab may:

- import `github.com/go-go-golems/pinocchio/pkg/evtstream` public packages,
- expose its own HTTP endpoints and UI shell,
- exercise the same public seams later transports will use.

Systemlab may not:

- import `pkg/webchat` internals,
- introduce SEM-specific substrate types into `evtstream`,
- reach around the public Hub/store/transport seams.

Current phases implemented:

- Phase 0 shell / status page
- Phase 1 command -> event -> projection lab
- Phase 2 ordering / ordinals lab

Current chapter coverage in Systemlab:

- Phase 0 through Phase 5 have long-form markdown chapters served by the app
- Phase 0, Phase 1, and Phase 2 also have working interactive pages
- Phase 3, Phase 4, and Phase 5 currently expose chapter-first scaffolds while their full interactive labs are still being built

## Frontend and chapter file layout

The Systemlab browser UI is intentionally split so future labs do not accumulate into one large inline HTML file, and the long-form intern chapters live as editable markdown beside the app:

- `static/index.html` — app shell only
- `static/app.css` — shared styling
- `static/partials/*.html` — page-level markup fragments
- `static/js/main.js` — bootstrap + navigation
- `static/js/pages/*.js` — per-page behavior
- `static/js/api.js` / `static/js/dom.js` — shared helpers
- `chapters/*.md` — long-form textbook chapters served by the app and rendered onto the matching phase pages

When adding a new lab, prefer adding:

- a new partial,
- a new page module,
- and, when needed, a matching markdown chapter in `chapters/`

instead of growing `index.html` or one global script.

Run locally:

```bash
make systemlab-run
```

Validation helpers:

```bash
make evtstream-test
make systemlab-build
make evtstream-boundary-check
```

Then open:

- `http://localhost:8091/`
