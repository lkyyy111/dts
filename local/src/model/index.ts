import { Database } from "@nozbe/watermelondb";
import SQLiteAdapter from "@nozbe/watermelondb/adapters/sqlite";
import LokiJSAdapter from "@nozbe/watermelondb/adapters/lokijs";

import schema from "@/model/schema";
import migrations from "@/model/migrations";
import User from "@/model/User";
import Space from "@/model/Space";
import SpaceMember from "@/model/SpaceMember";
import Photo from "@/model/Photo";
import Expense from "@/model/Expense";
import Comment from "@/model/Comment";
import Post from "@/model/Post";

const isTestEnv = process.env.NODE_ENV === "test";

const adapter = isTestEnv
  ? new LokiJSAdapter({
      schema,
      migrations,
    })
  : new SQLiteAdapter({
      schema,
      migrations,
      // The sync work is moving from a demo schema to the first top-level design
      // schema, so we intentionally start from a fresh local database name here.
      dbName: "dts_local_sync_v1",
      jsi: true,
      onSetUpError: (error) => {
        console.error("WatermelonDB 初始化失败:", error);
      },
    });

export const database = new Database({
  adapter,
  modelClasses: [User, Space, SpaceMember, Photo, Expense, Comment, Post],
});
