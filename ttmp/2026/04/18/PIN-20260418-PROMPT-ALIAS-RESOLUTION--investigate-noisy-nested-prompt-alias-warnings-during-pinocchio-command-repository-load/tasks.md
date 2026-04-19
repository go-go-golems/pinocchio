# Tasks

## Investigation baseline

- [x] Capture the warning shape and identify representative alias files
- [x] Trace the loader / repository path that emits the warning
- [x] Determine whether commands are inserted before aliases
- [x] Write up the initial root-cause note

## Analysis / design

- [x] Decide the intended contract for nested alias files
- [x] Write a detailed implementation plan for explicit alias paths
- [x] Expand the ticket docs to explain why load order is not the bug

## Implementation

- [x] Add shared Glazed alias-target parsing that accepts scalar and path-form `aliasFor`
- [x] Add helper methods so alias resolvers can distinguish legacy relative targets from explicit full paths
- [x] Update Glazed Cobra alias resolution to use the shared helper
- [x] Update Clay repository alias resolution to use the shared helper
- [x] Add focused regression tests for nested aliases targeting explicit full command paths
- [x] Migrate Pinocchio nested prompt alias fixtures to explicit path-form `aliasFor`
- [x] Re-run Pinocchio startup/help smoke and confirm the warnings are gone
- [x] Update ticket docs/tasks/changelog with the landed implementation details

