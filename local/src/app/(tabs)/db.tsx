import React, { useEffect, useState } from "react";
import { Q } from "@nozbe/watermelondb";
import { Button, FlatList, StyleSheet, Text, View } from "react-native";

import { database } from "@/model";
import Expense from "@/model/Expense";
import Space from "@/model/Space";
import SpaceMember from "@/model/SpaceMember";
import User from "@/model/User";
import { syncSpace } from "@/sync/sync";

const DEMO_SPACE_ID = "space_demo_sync";
const DEMO_USER_ID = "user_demo_sync";
const DEMO_SPACE_MEMBER_ID = `${DEMO_SPACE_ID}_${DEMO_USER_ID}`;

export default function DatabaseTestScreen() {
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [syncStatus, setSyncStatus] = useState("未同步");

  useEffect(() => {
    const expensesCollection = database.collections.get<Expense>("expenses");
    const subscription = expensesCollection
      .query(Q.where("space_id", DEMO_SPACE_ID))
      .observe()
      .subscribe((data) => {
        setExpenses(data);
      });

    return () => subscription.unsubscribe();
  }, []);

  const handleAddMockExpense = async () => {
    try {
      await database.write(async () => {
        await ensureDemoSyncContext();

        const expensesCollection =
          database.collections.get<Expense>("expenses");
        await expensesCollection.create((expense) => {
          expense.spaceId = DEMO_SPACE_ID;
          expense.payerId = DEMO_USER_ID;
          expense.amount = Math.floor(Math.random() * 5000) + 1000;
          expense.description = "离线测试：AA制午餐";
          expense.createdAt = new Date();
          expense.updatedAt = new Date();
        });
      });
    } catch (error) {
      console.error("写入失败:", error);
    }
  };

  const handleClearAll = async () => {
    try {
      await database.write(async () => {
        const expensesCollection =
          database.collections.get<Expense>("expenses");
        const allRecords = await expensesCollection
          .query(Q.where("space_id", DEMO_SPACE_ID))
          .fetch();
        const deleteOperations = allRecords.map((record) =>
          record.prepareMarkAsDeleted(),
        );
        await database.batch(...deleteOperations);
      });
    } catch (error) {
      console.error("清空失败:", error);
    }
  };

  const handleSyncCurrentSpace = async () => {
    try {
      setSyncStatus("同步中...");
      await syncSpace(DEMO_SPACE_ID);
      setSyncStatus("同步成功");
    } catch (error) {
      const message = error instanceof Error ? error.message : "未知同步错误";
      setSyncStatus(`同步失败: ${message}`);
      console.error("同步失败:", error);
    }
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>WatermelonDB 离线测试室</Text>
      <Text style={styles.subtitle}>当前空间: {DEMO_SPACE_ID}</Text>
      <Text style={styles.subtitle}>当前记录数: {expenses.length}</Text>
      <Text style={styles.subtitle}>同步状态: {syncStatus}</Text>

      <View style={styles.btnGroup}>
        <Button title="记一笔账" onPress={handleAddMockExpense} />
        <Button title="同步当前空间" onPress={handleSyncCurrentSpace} />
      </View>

      <View style={styles.btnGroup}>
        <Button title="清空全部" color="red" onPress={handleClearAll} />
      </View>

      <FlatList
        data={expenses}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <View style={styles.item}>
            <Text>{item.description}</Text>
            <Text style={styles.amount}>
              ￥{(item.amount / 100).toFixed(2)}
            </Text>
          </View>
        )}
      />
    </View>
  );

  async function ensureDemoSyncContext() {
    const usersCollection = database.collections.get<User>("users");
    const spacesCollection = database.collections.get<Space>("spaces");
    const spaceMembersCollection =
      database.collections.get<SpaceMember>("space_members");

    if (!(await findRecordOrNull(usersCollection, DEMO_USER_ID))) {
      await usersCollection.create((user) => {
        // Watermelon local create does not expose a typed id setter, so the
        // demo seed path assigns the sync identity through the raw record.
        setRawId(user, DEMO_USER_ID);
        user.nickname = "Local Demo User";
      });
    }

    if (!(await findRecordOrNull(spacesCollection, DEMO_SPACE_ID))) {
      await spacesCollection.create((space) => {
        setRawId(space, DEMO_SPACE_ID);
        space.name = "本地同步演示空间";
      });
    }

    if (
      !(await findRecordOrNull(spaceMembersCollection, DEMO_SPACE_MEMBER_ID))
    ) {
      await spaceMembersCollection.create((spaceMember) => {
        setRawId(spaceMember, DEMO_SPACE_MEMBER_ID);
        spaceMember.spaceId = DEMO_SPACE_ID;
        spaceMember.userId = DEMO_USER_ID;
      });
    }
  }
}

async function findRecordOrNull<T extends { id: string }>(
  collection: { find: (id: string) => Promise<T> },
  id: string,
): Promise<T | null> {
  try {
    return await collection.find(id);
  } catch {
    return null;
  }
}

function setRawId(model: object, id: string) {
  const modelWithRawId = model as { _raw: { id: string } };
  modelWithRawId._raw.id = id;
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 20, backgroundColor: "#f9f9f9" },
  title: {
    fontSize: 22,
    fontWeight: "bold",
    textAlign: "center",
    marginVertical: 10,
  },
  subtitle: { textAlign: "center", marginBottom: 8 },
  btnGroup: {
    flexDirection: "row",
    justifyContent: "space-around",
    marginBottom: 16,
  },
  item: {
    padding: 15,
    backgroundColor: "white",
    marginBottom: 10,
    borderRadius: 8,
    flexDirection: "row",
    justifyContent: "space-between",
  },
  amount: { fontWeight: "bold", color: "#e74c3c" },
});
