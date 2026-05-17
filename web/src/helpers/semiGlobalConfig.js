/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import semiGlobal from '@douyinfe/semi-ui/lib/es/_utils/semi-global';

const noMotion = { motion: false };

semiGlobal.config = {
  ...semiGlobal.config,
  overrideDefaultProps: {
    ...semiGlobal.config?.overrideDefaultProps,
    AutoComplete: noMotion,
    Cascader: noMotion,
    DatePicker: noMotion,
    Dropdown: noMotion,
    Modal: noMotion,
    Popconfirm: noMotion,
    Popover: noMotion,
    Select: noMotion,
    SideSheet: noMotion,
    TimePicker: noMotion,
    Tooltip: noMotion,
    TreeSelect: noMotion,
  },
};
