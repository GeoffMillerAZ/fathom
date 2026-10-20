### Added

- GraphQL and Prisma schemas link type references across their files: a GraphQL type a field, argument, union, implements list or type condition names binds the one file of the repository defining it, a Prisma field's model, view, composite type or enum the one file of its schema folder defining it; a name two files define binds nothing.
