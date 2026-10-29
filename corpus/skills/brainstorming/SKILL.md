---
name: brainstorming
description: Use before creative work, including features, components, behavior changes, and new plans in software or other fields.
---

<!-- Source: https://github.com/obra/superpowers/blob/bb92a77741419a4ab5f06e711a283343f1ada0c3/skills/brainstorming/SKILL.md -->
<!-- Approach selection and design approval source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/brainstorming/SKILL.md -->

# Brainstorming

Find out what the user wants and why before building. An agent can build a
working result from an incomplete understanding of the request.

People may still be working out what they want. Good questions help them
develop the idea. Understanding the purpose helps the agent make decisions
that the request does not explicitly cover.

## Assess the Request

Identify what the request establishes about the intended result and its purpose.

- Complete a quick, clear task when the request supplies every material detail,
  such as changing an icon to cornflower blue. State the completed change.
- Start a conversation when a material detail remains unclear.

## Open the Conversation

Use this order in the first message:

1. State that the user can skip the questions and request immediate work.
   If the user chooses that option, finish brainstorming. State any significant
   choice about platform, language, or medium in one plain sentence, then use
   the normal workflow to complete the request.
2. Ask one open question about a real moment or a concrete picture. For example,
   ask what happened the last time the need arose, or ask about the people who
   will attend an event.

Keep the first message limited to the skip option and the open question.

## Develop the Conversation

End each message with one question in a single sentence. Do not append a list
of example answers. Match the user's vocabulary and level of knowledge. Avoid
process jargon.

Write clearly and concisely. Present the most important findings first and
offer further details when useful.

Ask open questions first. The user's own account explains intent without
restricting the discussion to the agent's proposed options. Offer a short
menu when the user cannot answer. Propose something concrete when the user
requests a guess or a recommendation.

Establish the choices about medium, tools, materials, and resources early.
For an app, these include the platform, programming language, and libraries.
For art, these include the medium. Identify which choices the user wants to
make, which choices the agent can make, and which resources already exist.
Discuss choices when the user has strong preferences or relevant skills.
State sensible defaults plainly when the user delegates those choices.
Build on the tools and patterns that existing work already uses.

Record an agent-selected choice as the agent's decision. Attribute a decision
to the user only when the user made it.

Use these topics to guide exploration. Do not read the list aloud as questions:

- Establish the kind of result and what makes it distinct.
- Establish why the work matters now and what success means.
- Establish goals, non-goals, and outcomes that would constitute failure even
  if the result technically worked.
- Explore examples that the user liked or disliked.
- Inspect the current state and previous attempts.
- Identify the details that matter to the user.

Anchor questions in specifics. Ask about the last occurrence, the single most
important capability, or an outcome that would disappoint the user. A concrete
guess that the user can correct can help more than another abstract question.

Inspect the project, the user's relevant files and data, or the web when that
inspection would resolve a question. Report the relevant findings.

After understanding the request, present approaches in a separate message
before writing the design. Consider several real alternatives. Discard weak
alternatives, including those that conflict with the user's purpose or the
existing work. When alternatives remain, present the two or three remaining approaches. Ground each
approach in the project and explain its tradeoffs. Recommend one approach,
explain why, and ask the user to choose. If only one approach makes sense,
explain why the alternatives do not fit. Apply this comparison to how the
result will be built. First ask the user to explain the intended result.

Offer the visual companion when a visual would explain the subject better
than text. Relevant subjects include:

- Show an artifact the user will view, such as a screen, page, printout, or sign.
- Show a layout for a screen, room, or schedule.
- Show a sequence of operations or events.
- Show options that differ visually.
- Show a structure with a diagram, timeline, or map.

After the user accepts the companion, use mockups, comparisons, and clickable
throwaway prototypes to support the discussion.

Offer a spike when a quick throwaway build would clarify the request or test
feasibility. Complete the spike directly when the user requests one. A spike
skips the normal plan, implementation, and review process. Label its output as
throwaway and explain what the experiment established.

## Reflect the Understanding

Summarize each part once its purpose, audience, or behavior becomes clear.
Use about 200-300 words for that part, then ask about the next part. Alternate
summaries and questions throughout the conversation. Use the final summary to
cover any remaining details.

Distinguish the agent's assumptions from the user's stated requirements.
End each summary by asking what is wrong or missing.

Before presenting a summary, check whether it concerns an artifact the user
will view, such as a screen, page, schedule, or printout. If the user has not
received the visual companion offer, present the offer in its own message
first. Present the summary in the next message. Include a mockup beside the
summary after the user accepts the companion.

## Size the Work

Classify the work once the intended result becomes clear and the user chooses
an approach. State the size in plain words and allow the user to change it.

Choose the design format and subsequent workflow from the task size:

| Size | Full description | Subsequent work |
|---|---|---|
| A quick, clear task | Use the request itself. | Complete the task. |
| A small change | Write a short design document that records the discussion. | Obtain design approval before planning. |
| A project with a written design | Write a full design document. | Obtain design approval before planning. |

Include an Approaches section in each design document. Record the
approaches considered, the tradeoffs of each, the user's choice, and the reason
for that choice. Ask the user to choose an approach before writing the design.

Reassess the size if the scope grows during the task. Explain the change and
use the larger task's workflow.

For several independent projects, establish an order with the user and
brainstorm one project at a time.

## Write the Design

For a small change or a project, write a plain design document that a capable
builder in the domain can plan from without asking the user to repeat the discussion.
Include the following information:

- Explain the intended result and its purpose.
- State goals, non-goals, and outcomes that would constitute failure.
- State constraints.
- Explain the approaches, their tradeoffs, and the user's choice.
- Preserve the user's implementation choices as the user made them.
- Identify decisions the builder can make.

For a small change, cover each point in one or two lines and discuss the short
design in chat. The small change does not require a builder check.

For a software repository, save and commit the design under
`docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`. Ask for a location
outside a software repository. Follow the user's preferred location when
the user supplies one.

For a project, dispatch a fresh subagent with [builder-check-prompt.md](builder-check-prompt.md)
to check whether the document supplies enough information to plan the work.
Complete that check before presenting the final design in chat. Without a subagent tool,
review the document from the builder's perspective.

Resolve the reviewer's questions according to the available evidence:

- Add an answer that the conversation or project already supplies.
- Decide a minor detail when a capable builder could choose it without
  changing the intended result. Mark it as the agent's decision.
- Ask the user when only the user can answer and the answer changes the result.

Present the questions that require the user in one message, with the most
important question first. Update the document with the answers. Discuss the
design in chat, including its scope, behavior, constraints, tradeoffs, and
agent-selected decisions. Include enough detail in chat for the user to assess
and approve the design. Treat the saved document as an optional reference;
do not require the user to open it to review or approve the design.
Limit the builder check to one round of user questions.

Wait for the user to approve the design presented in chat before building.
Keep the saved document consistent with that approved design.
Building includes writing product code,
scaffolding, installing dependencies, creating projects, and invoking an
implementation skill. Reading and exploring can continue before approval.

The approval requirement ends when the user opts out of the questions. A spike
the user approves may proceed as a throwaway build. Treat retaining its output
as a new request and obtain design approval before continuing that work.

After design approval, create the implementation plan. For software, invoke
[writing-plans](../writing-plans/SKILL.md) as the next skill. Do not invoke
another skill before it.

## Check Common Mistakes

Use these checks when the discussion suggests a shortcut:

| Proposed shortcut | Required behavior |
|---|---|
| Offer options immediately to reduce effort. | Ask the user to explain the need before restricting the discussion to options. |
| Choose defaults without stating them. | State the defaults plainly and invite correction. |
| Explain the process before asking about the request. | Ask the substantive question directly. |
| Keep implementing a successful spike. | Treat retaining the spike's output as a new request and reassess its scope. |

## Use the Visual Companion

The visual companion uses a browser tab to present mockups, diagrams, and
prototypes. Acceptance makes the tool available for suitable questions.
Continue using text for questions that do not benefit from a visual.

Offer the companion the first time a visual would improve understanding.
Do not offer it at the start of every task. Put the offer in its own message:

> Would a browser tab with mockups help explain this choice?

If the user declines, continue in text. Do not offer the companion again
unless the user requests it.

Choose the presentation for each question. Use the browser for mockups,
layouts, diagrams, and visual comparisons. Use text for requirements, scope,
tradeoffs, and conceptual choices. A question about a user interface can
still require a text discussion.

After the user accepts, read [visual-companion.md](visual-companion.md) before
starting the server.
