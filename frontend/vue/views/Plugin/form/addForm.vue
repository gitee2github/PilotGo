<template>
  <div>
    <el-form :model="form" :rules="rules" ref="form" label-width="100px">
      <el-form-item label="插件名称:" prop="name">
        <el-input class="ipInput" controls-position="right" v-model="form.name" autocomplete="off"></el-input>
      </el-form-item>
      <el-form-item label="主机地址:" prop="url">
        <el-input class="ipInput" controls-position="right" v-model="form.url" autocomplete="off"></el-input>
      </el-form-item>
      <el-form-item label="展示类型:">
        <el-select v-model="form.plugin_type" placeholder="请选择插件展示类型">
          <el-option label="iframe" value="iframe"></el-option>
          <el-option label="micro-app" value="micro-app"></el-option>
        </el-select>
      </el-form-item>
    </el-form>

    <div class="dialog-footer">
      <el-button @click="handleCancel">取 消</el-button>
      <el-button type="primary" @click="handleAdd">确 定</el-button>
    </div>
  </div>
</template>

<script>
import { getPlugins, insertPlugin } from "@/request/plugin";
import _import from '../../../router/_import';
export default {
  data() {
    return {
      form: {
        name: "",
        url: "",
        plugin_type: "iframe",
      },
      rules: {
        name: [
          {
            required: true,
            message: '插件名称不能为空',
            trigger: "blur"
          },
        ],
        url: [
          {
            required: true,
            message: 'url不能为空',
            trigger: "blur"
          },
        ]
      },
    };
  },
  methods: {
    handleCancel() {
      this.$refs.form.resetFields();
      this.$emit("click");
    },
    handleAdd() {
      let pluginIndex = 100;
      getPlugins().then(res => {
        if (res.data.code === 200) {
          pluginIndex = res.data.data.length;
        }
      })
      this.$refs.form.validate((valid) => {
        if (valid) {
          insertPlugin(this.form)
            .then((res) => {
              if (res.data.code === 200) {
                // 更新dynamicRoutes数据
                this.$store.dispatch('SetDynamicRouters', []).then(() => {
                  // 更新左侧导航栏
                  this.$store.dispatch('GenerateRoutes');
                });

                this.$emit("click", "success");
                this.$message.success(res.data.msg);
                this.$refs.form.resetFields();
              } else {
                this.$message.error("添加插件失败：" + res.data.msg);
              }
            })
            .catch((res) => {
              console.log("添加插件失败：", res);
              this.$message.error("添加插件失败");
            });
        } else {
          this.$message.error("添加失败，请检查输入内容");
          return false;
        }
      });
    },
  },
};
</script>
<style scoped lang="scss"></style>