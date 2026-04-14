import { Model } from "@nozbe/watermelondb";
import { field } from "@nozbe/watermelondb/decorators";

export default class Space extends Model {
  static table = "spaces";

  @field("name") name!: string;
}
