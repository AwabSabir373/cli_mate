# Task 3: Agent Intelligence & Autonomy

**Goal:** Make the agent smarter at knowing when a task is actually finished, and allow it to self-escalate to stronger models.

## Context
If `cli_mate` finishes a response without a tool call, it assumes the task is done. It cannot switch models dynamically if a task proves too difficult.

## Requirements

### 1. Headless Completion Gates & Nudges
- **Problem:** The model may stop prematurely due to context limits, finishing mid-sentence, or it may forget to update its task tracker before concluding.
- **Solution:** Implement a completion gate check at the end of a turn that has no tool calls.
  - Check if the text ends in a continuation cue (e.g., `:` or `Let me check...`). If so, inject a system nudge: *"your message ended mid-step"*.
  - Check if there are pending items in the task/plan tracker. If so, inject a nudge: *"pending plan items remain — finish them, or mark them complete..."*.
  - Bound these nudges to a maximum count (e.g., `maxContinueNudges = 3`) to prevent infinite loops.

### 2. Mid-Run Model Escalation
- **Problem:** The user starts a task on a fast, cheap model (e.g., `gemini-1.5-flash`), but the task requires deep reasoning and the model gets stuck.
- **Solution:** 
  - Add a mechanism (perhaps a specific tool `request_model_escalation` or a specific return field) that allows the AI to say "I need a stronger model for this."
  - If this is detected during the tool loop, dynamically swap the underlying `providers.Provider` instance for the remainder of the execution loop (e.g., switch to `gemini-1.5-pro`).
  - Print a brief notice to the user that escalation occurred.
