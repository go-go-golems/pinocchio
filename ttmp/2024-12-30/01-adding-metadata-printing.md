I want to print out step metadata (for example, token usage, but also, which model has been used, etc...) in the pinocchio CLI and UI. I also want to export it to the json of the conversation.

For that, I need:

- actually storing the metadata into the conversation messages 
- maybe introduce a proper conversation struct instead of treating it as a list of messages, since there are overarching parts. Or maybe rename the Conversation tupe to Messages, and rename ConversationManager to Conversation. Because ConversationManager is actually the thing I want.

## StepPrinterFunc for the writing to writer

I am looking at the StepPrinterFunc in pkg/steps/ai/chat/conversation.go which prints out completion events
(received as watermill messagess) to the console. This received chat.Event has :

- EventImpl -> StepMetadata() and Metadata()
- Delta
- Completion