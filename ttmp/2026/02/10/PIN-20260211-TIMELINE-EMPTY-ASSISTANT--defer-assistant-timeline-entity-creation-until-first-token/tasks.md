# Tasks

## TODO


- [x] Trace cmd/pinocchio event flow that creates empty assistant timeline entity before first token
- [x] Identify all affected code paths (CLI timeline forwarder and webchat timeline projector)
- [x] Write analysis doc with root cause, recommended behavior, and follow-up implementation/test plan
- [x] Upload analysis document to reMarkable and record destination/path
- [x] Keep detailed diary with prompt context, commands, findings, and unresolved risks
- [x] Validate implementation with focused go tests for ui/webchat packages
- [x] Commit implementation and diary/changelog updates
- [x] Implement deferred assistant entity creation in StepChatForwardFunc (create on first assistant text)
