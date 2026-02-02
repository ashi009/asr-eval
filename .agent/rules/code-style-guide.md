# Coding Style & Naming Principles

These principles MUST be followed for all development in this project, regardless of the programming language. They are designed to maximize readability, maintainability, and architectural clarity.

## 1. Top-Down Organization (Breadth-First)
Organize files so the most important and abstract concepts appear first.
- **Data Structures/Types**: Define entry-point or "outer" types at the top. Referenced or "constituent" components should follow immediately after their parents.
- **Functions/Methods**: Group logic by abstraction and call order. High-level orchestrators go before their implementation-specific helper functions.
- **Tests**: Put entry-point tests (e.g., integration or main branch tests) at the top; move complex validation, mocking helpers, or utility logic to the bottom.

## 2. Strict Encapsulation
Minimize the surface area of the public API/Interface.
- **Private by Default**: Keep everything as restricted as possible (e.g., private in Class-based languages, unexported in Go, local in scripts) unless it is explicitly required across a boundary.
- **Export with Intent**: Only expose components that are part of the intended public contract for other modules or packages.

## 3. Naming (Information Density Metric)
Naming should be concise and proportional to the scope.
- **Scope-to-Description Ratio**:
    - **Small Scope (locals)**: Use short names where context is immediately visible and the scope is small.
    - **Large Scope (globals/exported)**: Use more descriptive names as they appear in a wider variety of contexts.
- **Information Density**: A name's length should not exceed its information content.
    - BAD: `getParametersAsNamedValuePairArray()`
    - GOOD: `queryParams()`
- **Precision over Verbosity**: Use precise, high-density words instead of long-winded ones. Accuracy is paramount; a verbose name is often wrong or redundant.
- **Reference**: While derived from Go best practices (see [Russ Cox: Names](https://research.swtch.com/names)), these apply to all clean code architectures.
