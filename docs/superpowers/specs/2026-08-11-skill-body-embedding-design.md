# Skill Body Embedding Design

The corpus compiler will inline skill bodies through `{{.SkillBody "name"}}`.

`SkillBody` removes the embedded skill's front matter and expands its template in the caller's output context. Existing `RuleBody`, `Rule`, `Rules`, and `Skill` behavior remains unchanged.

One expansion engine will resolve rule and skill bodies. It will track typed nodes such as `skill:a` and `rule:b`. Re-entering an active node will fail with the complete cycle path.

Rendering will reject missing bodies, invalid names, direct cycles, same-type cycles, and cross-type cycles before writing output. Focused tests will prove successful nesting and each failure class.
