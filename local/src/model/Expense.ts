import { Model } from "@nozbe/watermelondb";
import { date, field } from "@nozbe/watermelondb/decorators";

export default class Expense extends Model {
  static table = "expenses";

  @field("space_id") spaceId!: string;
  @field("payer_id") payerId!: string;
  @field("amount") amount!: number;
  @field("description") description!: string;
  @date("created_at") createdAt!: Date;
  // Keep the decorator name aligned with data-design.md.
  @date("upadted_at") updatedAt!: Date;
}
