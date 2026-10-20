### Added

- GraphQL and Prisma schemas link type references across their files (L3 excellent, measured against graphql-js on octokit/graphql-schema, gqlgen and vendure, and against the Prisma schema engine on cal.com, web-app-template and dub): a GraphQL type a field, argument, union, implements list or type condition names binds the one file of the repository defining it, a Prisma field's model, view, composite type or enum the one file of its schema folder defining it; a name two files define binds nothing. GraphQL and Prisma files ingested before re-read once.
