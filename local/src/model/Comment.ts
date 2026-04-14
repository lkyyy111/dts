import { Model } from "@nozbe/watermelondb";
import { date, field } from "@nozbe/watermelondb/decorators";

export default class Comment extends Model {
  static table = "comments";

  @field("space_id") spaceId!: string;
  @field("content") content!: string;
  @field("commenter_id") commenterId!: string;
  @field("post_id") postId!: string;
  @field("commented_at") commentedAtMs!: number;
  @date("created_at") createdAt!: Date;
  @date("updated_at") updatedAt!: Date;
}
