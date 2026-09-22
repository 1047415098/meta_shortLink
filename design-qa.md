# Free novel H5 design QA

Date: 2026-09-21

## Screens checked

- Home at 390 × 844: compact header, centered portrait feature, ranking grid, two-column story cards, fixed bottom navigation.
- Search at 390 × 844: back control, search input, debounced query, results and empty guidance.
- Story/reader at 390 × 844: blurred cover hero, metadata, start button, readable body width, chapter controls.
- Contents drawer: modal semantics, focus entry, Escape/backdrop/close paths, current chapter marker, no lock icons.
- Desktop at 1280 px: centered content and no horizontal overflow.

## Interaction checks

- Home → search → result → reader completed.
- Chapter 1 → drawer → chapter 2 completed.
- Refresh retained chapter 2 through local reading progress.
- DOM width checks reported no horizontal overflow at mobile or desktop sizes.

## Notes

- Visual QA used the production Vue build with a temporary local read-only fixture API because the updated Docker image could not pull `node:22-alpine` from Docker Hub during the test window.
- The temporary fixture service was stopped and the pre-existing local application container was restored after QA.

final result: passed
