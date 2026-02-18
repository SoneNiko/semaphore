create table `project__template_inventory` (
    `id` integer primary key autoincrement,
    `template_id` int not null,
    `inventory_id` int not null,

    unique (`template_id`, `inventory_id`),
    foreign key (`template_id`) references project__template(`id`) on delete cascade,
    foreign key (`inventory_id`) references project__inventory(`id`) on delete cascade
);

insert into `project__template_inventory` (`template_id`, `inventory_id`)
select `id`, `inventory_id`
from `project__template`
where `inventory_id` is not null;
