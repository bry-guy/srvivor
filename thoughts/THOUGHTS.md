# Thoughts Directory Structure

This directory contains architecture details, research reports, implementation plans and design decisions for the srvivor project.

## Structure

- design/ Product designs about how the application is expected to work from a user experience
- architecture/ Architectural details about how the project is structure, where the core components are, business logic, testing, development, deployment, etc
- tickets/ These are issues, feature requests, or differnces between the architecutre and the codebase.
- research/ Results from performing codebase, thoughts and web search and analysis, these are used to guide planning
- plans/ Related directly to a ticket and a research file, contains specific details of what needs to be implemented and how to test it
- reviews/ Related directly to a plan and contains analysis of how well the plan was implemented, ensures it was correct and complete, and documents any drift that occured
- archive/ Documents that have been removed from circulation as they are not relevant, likely due to code churn or changing architecture details.
- meta/ About thoughts usages and practices themselves.

## Research In Thoughts

When researching the thoughts, the important folders to search through are design, architecture and reseach. They both contain the most high level information and analysis of the codebase.
Second to that are the plans and reviews. These contain previous implementation history, though these may be archived once they are deemed to be out of date due to code churn.
Tickets may be scanned to see what is already scheduled for implemenation. This may be useful when we want to determine if the current ticket has overlap or dependencies with other tickets. Meta can be used to understand how thoughts are supposed to work over time.

**IMPORTANT** archive/ is to be avoided by all research tasks. These have been determined to no longer be relevant and contain misleading information. They are for historical record and subject to future deletion or summarization.

## Usage

Create markdown files in these directories to document:
- Architecture designs and decisions
- Issues, feature requests and architecture deltas
- Research of codebase, thoughts and web documentation
- Implementation plans and reviews

**IMPORTANT** design/ is not to be modified.
