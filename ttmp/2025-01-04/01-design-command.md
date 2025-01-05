## Goal: make it easier to write code to do things with LLM

I want to make it easier to run GeppettoCommands from code.
So far, it was all geared to wards running command line commands with this, 
and so the code for GeppettoCommand is heavily intertwined with TUI stuff.

I need a bunch of things for that:
- nicely create and render the initial conversation (that might be all i need the command for, really)
- quickly create a chat step to run inference
    - that is actually completely separate from the geppetto command as such
- use geppetto commands for REST API and also quickly allow them to be served as SSE streams, etc..
    - this uses the router

## Problems right now

- CommandContext used by GeppettoCommand is a mess
- HelpersSettings are relevant to TUI stuff
- We shouldn't need an embedding slayer if not using embeddings
- ConversationContext is mostly a big constructor for ConversationManager
- ConversationManager does autosave, not sure if that's that big of a problem
    - BRAINSTORM: In a more generic setup, there would be events that would allow the autosave functionality to get triggered when the conversation changes
        - BRAINSTORM: is there a more generic way to combine all these different events we want in the ecosystem? (so, partial completion events, but also changes to the conversation itself. Should they all go over the same bus or should they use different pubsub, or maybe just different topics?)
- HelperSettings really should be called PinocchioTerminalSettings or something, or maybe just PinocchioSettings
- GeppettoCommand is kind of tightly linked to pinocchio, maybe it is possible to have a pinocchio command do all the terminal stuff, and also have a simple GeppettoCommand in the main framework to do prompt rendering / easy chat step setup

- EmbeddingsSettings should not be part of StepSettings
    - the thing though is that embeddings commands still need the provider information (base url, api keys)
    - we should split StepSettings into: 
        - ProviderSettings
        - ChatSettings
        - EmbeddingSettings

## Current state of CommandContext

-> we should ask claude for ways to refactor it. IF we only use it for PinocchioCommand, might as well just merge it back

- can be created from StepSettings (NewCommandContextFromSettings)
    - still uses HelperSettings, though, but that could be passed as an option
    - creates a router, which also should be optional
    - passes in a standard StepFactory (the ai.StandardStepFactory)
- I don't think we need that method at all, since it really just wraps around the actual constructor by prepopulating a few options, but not the right options, in some way


- StartInitialStep:
    - does create a step with published topic "chat", always
    - it gets the conversation from the conversation manager, and then kickstarts the chat step with it

- CommandContext handles the whole chat/nonchat, interactive stuff
    -> I think in a purely programmatic context, we don't even need the commandContext and should just use the step directly
    - actually it does the Router handling, which is pretty useful. So I wonder if we should create a terminal running command context? or what a better name would be for something to run a step and manage an additional router. Or maybe this could all be in Geppetto Command. I do think that renaming GeppettoCommand into PinocchioCommand, and then creating a clean separate GeppettoCommand would be a good start. We can always have PinocchioCommand reuse that in the future.

## Running a chat step

To run a step, we need:
- an initial conversation
    - NOTE: here the API to create and manage a conversation, through a ConversationManager, is not too bad
    - NOTE: but we also want to be able to quickly load templates / commands from a yaml file and render them
- creating a chat step, with StepSettings
    - StandardStepFactory, and then binding the conversation.
    - should this be a helper method?


## Step for the refactoring

- merge the web-ui before doing a bigger refactor
    - NOTE: maybe merge eval as well? But I remember breaking some things in there

- rename GeppettoCommand to PinocchioCommand
- create a separate GeppettoCommand that only renders out the string

- allow an empty router

- split StepSettings into smaller settings

- helper functions to load provider settings, chat settings, etc...