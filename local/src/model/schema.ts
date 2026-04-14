import { appSchema, tableSchema } from "@nozbe/watermelondb";

export default appSchema({
  version: 1,
  tables: [
    tableSchema({
      name: "users",
      columns: [{ name: "nickname", type: "string" }],
    }),
    tableSchema({
      name: "spaces",
      columns: [{ name: "name", type: "string" }],
    }),
    tableSchema({
      name: "space_members",
      columns: [
        { name: "space_id", type: "string", isIndexed: true },
        { name: "user_id", type: "string", isIndexed: true },
      ],
    }),
    tableSchema({
      name: "photos",
      columns: [
        { name: "space_id", type: "string", isIndexed: true },
        { name: "uploader_id", type: "string" },
        { name: "remote_url", type: "string", isOptional: true },
        { name: "post_id", type: "string", isIndexed: true },
        { name: "shoted_at", type: "number" },
        { name: "created_at", type: "number" },
        { name: "updated_at", type: "number" },
      ],
    }),
    tableSchema({
      name: "expenses",
      columns: [
        { name: "space_id", type: "string", isIndexed: true },
        { name: "payer_id", type: "string" },
        { name: "amount", type: "number" },
        { name: "description", type: "string" },
        { name: "created_at", type: "number" },
        // Keep the raw column name aligned with data-design.md.
        { name: "upadted_at", type: "number" },
      ],
    }),
    tableSchema({
      name: "comments",
      columns: [
        { name: "space_id", type: "string", isIndexed: true },
        { name: "content", type: "string" },
        { name: "commenter_id", type: "string" },
        { name: "post_id", type: "string", isIndexed: true },
        { name: "commented_at", type: "number" },
        { name: "created_at", type: "number" },
        { name: "updated_at", type: "number" },
      ],
    }),
    tableSchema({
      name: "posts",
      columns: [
        { name: "space_id", type: "string", isIndexed: true },
        { name: "created_at", type: "number" },
        { name: "updated_at", type: "number" },
      ],
    }),
  ],
});
