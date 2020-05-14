<template>
  <fragment>
    <v-row>
      <v-col
        cols="12"
        md="4"
        class="pt-0"
      >
        <v-card class="pa-2">
          <v-list-item three-line>
            <v-list-item-content>
              <div class="overline mb-4">
                Registered Devices
              </div>
              <v-list-item-title class="headline mb-1">
                {{ stats.registered_devices }}
              </v-list-item-title>
              <v-list-item-subtitle>Registered devices into the tenancy account</v-list-item-subtitle>
            </v-list-item-content>

            <v-list-item-avatar
              tile
              size="80"
            >
              <v-icon x-large>
                devices
              </v-icon>
            </v-list-item-avatar>
          </v-list-item>

          <v-card-actions v-if="stats.registered_devices == 0">
            <Welcome :screen-welcome="true" />
          </v-card-actions>

          <v-card-actions>
            <DeviceAdd />
            <v-btn
              text
              @click="$store.dispatch('modals/showAddDevice', true)"
            >
              Add Device
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>

      <v-col
        cols="12"
        md="4"
        class="pt-0"
      >
        <v-card class="pa-2">
          <v-list-item three-line>
            <v-list-item-content>
              <div class="overline mb-4">
                Online Devices
              </div>
              <v-list-item-title class="headline mb-1">
                {{ stats.online_devices }}
              </v-list-item-title>
              <v-list-item-subtitle>Devices are online and ready for connecting</v-list-item-subtitle>
            </v-list-item-content>

            <v-list-item-avatar
              tile
              size="80"
            >
              <v-icon x-large>
                devices
              </v-icon>
            </v-list-item-avatar>
          </v-list-item>

          <v-card-actions>
            <v-btn
              to="/devices"
              text
            >
              View all Devices
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>

      <v-col
        cols="12"
        md="4"
        class="pt-0"
      >
        <v-card class="pa-2">
          <v-list-item three-line>
            <v-list-item-content>
              <div class="overline mb-4">
                Active Sessions
              </div>
              <v-list-item-title class="headline mb-1">
                {{ stats.active_sessions }}
              </v-list-item-title>
              <v-list-item-subtitle>Active SSH Sessions opened by users</v-list-item-subtitle>
            </v-list-item-content>

            <v-list-item-avatar
              tile
              size="80"
            >
              <v-icon x-large>
                history
              </v-icon>
            </v-list-item-avatar>
          </v-list-item>

          <v-card-actions>
            <v-btn
              to="/sessions"
              text
            >
              View all Sessions
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </fragment>
</template>

<script>
import DeviceAdd from '@/components/device/DeviceAdd.vue';
import Welcome from '@/components/welcome/Welcome.vue';

export default {
  name: 'Dashboard',

  components: {
    DeviceAdd,
    Welcome,
  },

  computed: {
    stats() {
      return this.$store.getters['stats/stats'];
    }
  },

  async created() {
    await this.$store.dispatch('stats/get');
  }
};
</script>
