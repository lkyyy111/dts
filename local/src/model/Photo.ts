import { Model } from "@nozbe/watermelondb";
import { date, field } from "@nozbe/watermelondb/decorators";

export default class Photo extends Model {
  static table = "photos";

  @field("space_id") spaceId!: string;
  @field("uploader_id") uploaderId!: string;
  @field("remote_url") remoteUrl!: string | null;
  @field("post_id") postId!: string;
  @field("shoted_at") shotedAtMs!: number;
  @date("created_at") createdAt!: Date;
  @date("updated_at") updatedAt!: Date;
}
