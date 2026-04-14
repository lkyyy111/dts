import { Model } from "@nozbe/watermelondb";
import { field } from "@nozbe/watermelondb/decorators";

export default class SpaceMember extends Model {
  static table = "space_members";

  @field("space_id") spaceId!: string;
  @field("user_id") userId!: string;
}
