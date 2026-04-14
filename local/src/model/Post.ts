import { Model } from "@nozbe/watermelondb";
import { date, field } from "@nozbe/watermelondb/decorators";

export default class Post extends Model {
  static table = "posts";

  @field("space_id") spaceId!: string;
  @date("created_at") createdAt!: Date;
  @date("updated_at") updatedAt!: Date;
}
